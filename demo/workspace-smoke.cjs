'use strict';

const assert = require('node:assert/strict');
const { createHash, randomBytes, randomUUID } = require('node:crypto');
const { readFile } = require('node:fs/promises');
const { chromium } = require('playwright');

const files = 'http://localhost:5080';
const issuer = 'http://localhost:8080/realms/subsoil';
const registry = 'http://localhost:5001';
const editor = 'http://localhost:5173/';
const password = 'Demo-pass-238!example';
const notes = 'application/vnd.subsoil.demo.notes.v1';
const review = 'application/vnd.subsoil.demo.review.v1+json';
const workPackage = 'application/vnd.subsoil.demo.work-package.v1';

async function verifyEmail(page, email) {
  for (let tries = 0; tries < 30; tries++) {
    const inbox = await (await fetch('http://localhost:8025/api/v1/messages')).json();
    const item = inbox.messages?.find(message => message.To.some(recipient => recipient.Address.toLowerCase() === email));
    if (item) {
      const message = await (await fetch('http://localhost:8025/api/v1/message/' + item.ID)).json();
      const link = message.Text.match(/https?:\/\/[^\s<]+\/login-actions\/action-token\?[^\s<]+/);
      assert(link, 'Keycloak sent an email verification link');
      await page.goto(link[0]);
      const confirm = page.locator('#kc-info-message a[href*="/login-actions/action-token"]');
      if (await confirm.isVisible()) await confirm.click();
      return;
    }
    await new Promise(resolve => setTimeout(resolve, 500));
  }
  assert.fail('No verification email arrived at ' + new URL(page.url()).pathname +
    ' (' + (await page.locator('h1').allTextContents()).join(' ') + ')');
}

function tokenResponse(page) {
  return page.waitForResponse(reply => reply.url() === issuer + '/protocol/openid-connect/token' && reply.request().method() === 'POST');
}

async function checkTheme(page) {
  await page.waitForFunction(() =>
    getComputedStyle(document.documentElement).getPropertyValue('--subsoil-green').trim() === '#234f3c');
}

async function signUp(browser, label) {
  const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
  const page = await context.newPage();
  const name = label.toLowerCase() + '-' + randomUUID().slice(0, 8);
  const email = name + '@example.test';
  await page.goto(files + '/explore');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await checkTheme(page);
  await page.getByRole('link', { name: 'Register' }).click();
  await checkTheme(page);
  const form = page.locator('#kc-register-form');
  await form.locator('#firstName').fill(label);
  await form.locator('#lastName').fill('Workspace');
  await form.locator('#email').fill(email);
  await form.locator('#username').fill(name);
  if (await form.locator('#password').isVisible()) {
    await form.locator('#password').fill(password);
    await form.locator('#password-confirm').fill(password);
  }
  await form.locator('[type=submit]').click();
  const authenticated = tokenResponse(page);
  const provisioned = page.waitForResponse(reply => reply.url() === files + '/workspace');
  // Report the page that blocked setup without printing action links or tokens.
  const observed = Promise.all([authenticated, provisioned]);
  observed.catch(() => {});
  await verifyEmail(page, email);
  const update = page.locator('#kc-passwd-update-form');
  await update.waitFor({ timeout: 5000 }).catch(() => {});
  if (await update.isVisible()) {
    await update.locator('#password-new').fill(password);
    await update.locator('#password-confirm').fill(password);
    await update.locator('[type=submit]').click();
  }
  if (!page.url().startsWith(files + '/explore')) {
    const heading = (await page.locator('h1').allTextContents()).join(' ');
    console.log('Email confirmation returned to ' + new URL(page.url()).pathname + ' (' + heading.trim() + ').');
    await page.goto(files + '/explore');
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    const login = page.locator('#kc-form-login');
    await login.waitFor({ timeout: 4000 }).catch(() => {});
    if (await login.isVisible()) {
      await login.locator('#username').fill(name);
      await login.locator('#password').fill(password);
      await login.locator('[type=submit]').click();
    }
  }
  const [reply, workspace] = await observed;
  assert.equal(reply.status(), 200, 'Workspace PKCE token');
  const granted = await reply.json();
  const token = granted.access_token;
  if (workspace.status() !== 200) {
    const claims = JSON.parse(Buffer.from(token.split('.')[1], 'base64url'));
    console.log('Workspace token details', {
      scope: claims.scope, responseScope: granted.scope, audience: claims.aud,
      hasWorkspace: claims.access?.some(entry => entry.name.startsWith('workspaces/'))
    });
  }
  assert.equal(workspace.status(), 200, await workspace.text());
  const settings = await workspace.json();
  await page.getByRole('link', { name: 'My workspace', exact: false }).waitFor();
  console.log(label + ' signed up, verified email and received a private workspace.');
  return { context, page, token, name, ...settings };
}

async function http(origin, path, token, method = 'GET', body, headers = {}) {
  return fetch(origin + path, {
    method, body, headers: { ...headers, Authorization: 'Bearer ' + token }, redirect: 'manual'
  });
}

async function register(account, name, callback, handles) {
  const reply = await http(files, '/workspaces/' + account.id + '/apps', account.token, 'POST',
    JSON.stringify({ name, redirect_uris: [callback], launch_url: callback, handles }),
    { 'Content-Type': 'application/json' });
  assert.equal(reply.status, 201, await reply.clone().text());
  return reply.json();
}

async function open(app, workspace, type) {
  await workspace.page.getByRole('link', { name: 'My workspace', exact: false }).click();
  await workspace.page.getByRole('link', { name: 'notes-v1', exact: true }).click();
  const popup = workspace.context.waitForEvent('page');
  await workspace.page.getByRole('button', { name: type === 'edit' ? 'Edit with…' : 'Open with…' }).click();
  const page = await popup;
  await page.waitForURL(url => url.origin + url.pathname === app.launch_url);
  return page;
}

async function authorize(account, app, callback, scope, expectedStatus = 200) {
  const page = await account.context.newPage();
  try {
    const returned = page.waitForRequest(request => request.isNavigationRequest() && request.url().startsWith(callback + '?'));
    returned.catch(() => {});
    const verifier = randomBytes(32).toString('base64url');
    const state = randomUUID();
    const request = new URLSearchParams({
      client_id: app.client_id, redirect_uri: callback, scope, state,
      response_type: 'code', code_challenge_method: 'S256',
      code_challenge: createHash('sha256').update(verifier).digest('base64url'),
      resource: registry,
    });
    await page.goto(issuer + '/protocol/openid-connect/auth?' + request);
    const accept = page.locator('#kc-oauth #kc-login');
    if (await accept.isVisible()) await accept.click();
    const reply = new URL((await returned).url()).searchParams;
    assert.equal(reply.get('error'), null, reply.get('error_description') || 'authorization succeeded');
    assert.equal(reply.get('state'), state);
    assert.equal(reply.get('iss'), issuer);
    assert(reply.get('code'), 'authorization code returned to the exact callback');
    const token = await fetch(issuer + '/protocol/openid-connect/token', {
      method: 'POST', body: new URLSearchParams({
        grant_type: 'authorization_code', client_id: app.client_id, redirect_uri: callback,
        code: reply.get('code'), code_verifier: verifier, resource: registry,
      }),
    });
    assert.equal(token.status, expectedStatus, 'workspace app code exchange');
    return token.json();
  } finally {
    await page.close();
  }
}

async function checkProjectFiles(browser, account) {
  const context = await browser.newContext({
    permissions: ['clipboard-read', 'clipboard-write'],
    viewport: { width: 390, height: 844 },
  });
  try {
    const page = await context.newPage();
    const browserTokenRequests = [];
    page.on('request', request => {
      if (request.url() === issuer + '/protocol/openid-connect/token') browserTokenRequests.push(request);
    });
    await page.goto(files + '/');
    await page.getByRole('link', { name: 'Sign in', exact: true }).click();
    await page.locator('#username').fill('reader');
    await page.locator('#password').fill('reader-demo-password');
    await page.locator('#kc-login').click();
    await page.getByRole('heading', { name: 'Project files' }).waitFor();
    assert.equal(browserTokenRequests.length, 0, 'Go exchanges the CDE authorization code');
    const session = (await context.cookies(files)).find(cookie => cookie.name === 'subsoil_cde');
    assert(session?.httpOnly, 'CDE session cookie is HTTP-only');
    assert.equal((await context.request.post(files + '/sign-out', { form: { csrf: 'invalid' } })).status(), 403);

    const [fragment] = await Promise.all([
      page.waitForResponse(response => response.url().includes('version=sha256')),
      page.getByRole('link', { name: 'Level 02 coordination', exact: true }).click(),
    ]);
    assert.equal(fragment.request().headers()['hx-request'], 'true');
    const panel = page.getByRole('region', { name: 'Level 02 coordination' });
    const header = panel.locator('.panel-header');
    await header.getByText('Released by Sam Lee').waitFor();
    assert.equal(await panel.locator('.release-file').count(), 5);
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await page.getByRole('link', { name: /Released by Priya Shah/ }).click();
    await header.getByText('Released by Priya Shah').waitFor();
    await page.goBack();
    await header.getByText('Released by Sam Lee').waitFor();
    await page.goForward();
    await header.getByText('Released by Priya Shah').waitFor();
    await page.reload();
    await header.getByText('Released by Priya Shah').waitFor();

    await page.getByRole('button', { name: 'Copy release link' }).click();
    await page.getByRole('status').getByText('Link copied').waitFor();
    const link = await page.evaluate(() => navigator.clipboard.readText());
    assert.match(new URL(link).searchParams.get('version'), /^sha256:[a-f0-9]{64}$/);
    const [download] = await Promise.all([
      page.waitForEvent('download'),
      page.getByRole('link', { name: 'Download level-02-plan.pdf' }).click(),
    ]);
    assert.equal(download.suggestedFilename(), 'level-02-plan.pdf');
    assert.deepEqual(await readFile(await download.path()), await readFile('demo/work-package/plan.pdf'));

    await page.getByRole('button', { name: 'Sign out' }).click();
    await page.getByRole('heading', { name: 'Sign in to Subsoil' }).waitFor();
    await page.goto(link);
    await page.getByRole('link', { name: 'Sign in', exact: true }).click();
    await header.getByText('Released by Priya Shah').waitFor();
    assert.equal(page.url(), link);
  } finally {
    await context.close();
  }

  const page = await account.context.newPage();
  try {
    await page.goto(files + '/');
    await page.getByRole('link', { name: 'Sign in', exact: true }).click();
    await page.getByRole('alert').getByText('Ask your project administrator for access.', { exact: false }).waitFor();
    await page.getByRole('link', { name: 'My workspace' }).click();
    await page.getByRole('link', { name: 'notes-v2', exact: true }).waitFor();
    console.log('The Go CDE restored an exact work-package release after sign-in, browsed its history with HTMX, and enforced project access.');
    await checkPackageApps(page, account);
  } finally {
    await page.close();
  }
}

async function checkPackageApps(page, account) {
  await page.getByRole('button', { name: 'Add sample work package' }).click();
  await page.getByRole('link', { name: 'Level 02 coordination', exact: true }).waitFor();
  await page.getByRole('link', { name: 'Workspace apps' }).click();
  await page.getByLabel('App name').fill('Package viewer');
  const callback = editor + 'index.html';
  await page.getByLabel('Callback URLs').fill(callback);
  await page.getByLabel('Launch URL').fill(callback);
  await page.getByLabel('View package types').fill(workPackage);
  await page.getByLabel('View file types').fill('application/pdf');
  await page.getByRole('button', { name: 'Register app' }).click();
  const configText = await page.locator('.app-registrations pre:visible').textContent();
  const config = JSON.parse(configText);
  assert.deepEqual(config.handles, [{ artifact_type: workPackage, intents: ['view'] }]);
  assert.deepEqual(config.file_handles, [{ media_type: 'application/pdf', intents: ['view'] }]);
  const pdfOnly = await http(files, '/workspaces/' + account.id + '/apps', account.token, 'POST',
    JSON.stringify({
      name: 'PDF reader', redirect_uris: [editor], launch_url: editor,
      file_handles: [{ media_type: 'application/pdf', intents: ['view'] }],
    }), { 'Content-Type': 'application/json' });
  assert.equal(pdfOnly.status, 201, await pdfOnly.clone().text());
  assert.deepEqual((await pdfOnly.json()).handles, []);

  await page.getByRole('link', { name: 'My workspace' }).click();
  await page.getByRole('link', { name: 'Level 02 coordination', exact: true }).click();
  const panel = page.getByRole('region', { name: 'Level 02 coordination' });
  await panel.getByText('Open package with…', { exact: true }).click();
  const whole = panel.locator('.version-actions').getByRole('link', { name: 'Open with Package viewer' });
  assert.equal(await panel.locator('.version-actions').getByRole('link', { name: 'Open with PDF reader' }).count(), 0);
  const selectedRow = panel.locator('.release-file').filter({ hasText: 'coordination-checklist.pdf' });
  await selectedRow.getByText('Open with…', { exact: true }).click();
  await selectedRow.getByRole('link', { name: 'Open with PDF reader' }).waitFor();
  const file = selectedRow.getByRole('link', { name: 'Open with Package viewer' });

  async function inspectLaunch(link) {
    const response = await page.request.get(new URL(await link.getAttribute('href'), files).href, { maxRedirects: 0 });
    assert.equal(response.status(), 303);
    const address = new URL(response.headers().location);
    const fragment = new URLSearchParams(address.hash.slice(1)).get('handoff');
    return { address, payload: JSON.parse(Buffer.from(fragment, 'base64url')) };
  }

  const packageLaunch = await inspectLaunch(whole);
  const fileLaunch = await inspectLaunch(file);
  assert.equal(packageLaunch.address.search, '');
  assert.equal(fileLaunch.address.searchParams.get('layer'), '2');
  assert.deepEqual(fileLaunch.payload, packageLaunch.payload, 'both launches use the same oci-handoff/1 message');
  assert.deepEqual(Object.keys(fileLaunch.payload).sort(), ['intent', 'protocol', 'source']);
  assert.equal(fileLaunch.payload.protocol, 'oci-handoff/1');
  assert.equal(fileLaunch.payload.source.repository, account.repository);
  const wrongFile = new URL(await file.getAttribute('href'), files);
  wrongFile.searchParams.set('layer', '0');
  assert.equal((await page.request.get(wrongFile.href, { maxRedirects: 0 })).status(), 400);

  async function openViewer(link) {
    const [viewer] = await Promise.all([page.context().waitForEvent('page'), link.click()]);
    const pulls = [];
    viewer.on('request', request => {
      const url = new URL(request.url());
      if (url.origin === registry && url.pathname.startsWith('/v2/' + account.repository + '/')) {
        pulls.push(url.pathname);
      }
    });
    const connect = viewer.locator('#settings');
    await connect.waitFor();
    if ((await connect.getAttribute('open')) === null) {
      await connect.locator('summary').click();
    }
    await viewer.getByLabel('App configuration').fill(configText);
    await viewer.getByRole('button', { name: 'Connect', exact: true }).click();
    await viewer.getByRole('button', { name: 'Sign in to open' }).click();
    const consent = viewer.locator('#kc-oauth #kc-login');
    await Promise.race([
      consent.waitFor({ timeout: 6000 }).catch(() => {}),
      viewer.getByRole('heading', { name: 'Level 02 coordination' }).waitFor({ timeout: 6000 }).catch(() => {}),
    ]);
    if (await consent.isVisible()) await consent.click();
    await viewer.getByRole('heading', { name: 'Level 02 coordination' }).waitFor();
    return { viewer, pulls };
  }

  const wholeApp = await openViewer(whole);
  assert.equal(await wholeApp.viewer.locator('#package-files button').count(), 5);
  assert(wholeApp.pulls.some(path => path.endsWith('/manifests/' + packageLaunch.payload.source.digest)));
  await wholeApp.viewer.close();

  const fileApp = await openViewer(file);
  await fileApp.viewer.getByRole('heading', { name: 'coordination-checklist.pdf' }).waitFor();
  assert.equal(await fileApp.viewer.locator('#package-files button').count(), 2);
  const manifestResponse = await http(registry, '/v2/' + account.repository + '/manifests/' + fileLaunch.payload.source.digest, account.token);
  const manifest = await manifestResponse.json();
  assert.deepEqual(fileApp.pulls.filter(path => path.includes('/blobs/')), [
    '/v2/' + account.repository + '/blobs/' + manifest.layers[2].digest,
  ]);
  await fileApp.viewer.close();
  console.log('One app registered both capabilities, read the complete manifest, and fetched exactly the selected PDF in a file launch.');
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  try {
    const alice = await signUp(browser, 'Alice');
    const bob = await signUp(browser, 'Bob');
    assert.notEqual(alice.repository, bob.repository);
    const account = await fetch(issuer + '/account/');
    assert.equal(account.status, 200);
    const logo = (await account.text()).match(/"resourceUrl": "([^"]+)"/);
    assert(logo, 'Keycloak supplied an account theme resource path');
    const brand = await fetch(new URL(logo[1] + '/img/subsoil-logo.svg', issuer));
    assert.equal(brand.headers.get('content-type'), 'image/svg+xml');
    await alice.page.getByText('Workspace apps', { exact: true }).click();
    const registration = alice.page.locator('#register-app');
    await registration.getByLabel('App name').fill('Text editor');
    const publishCallback = editor + 'index.html';
    await registration.getByLabel('Callback URLs').fill(editor + '\n' + publishCallback);
    await registration.getByLabel('Artifact types to view').fill(notes + ', ' + review);
    await registration.getByLabel('Artifact types to edit').fill(notes);
    const registered = alice.page.waitForResponse(reply =>
      reply.url() === files + '/workspaces/' + alice.id + '/apps' && reply.request().method() === 'POST');
    await registration.getByRole('button', { name: 'Register app' }).click();
    const registeredReply = await registered;
    assert.equal(registeredReply.status(), 201, await registeredReply.text());
    const app = await registeredReply.json();
    assert.deepEqual(app.redirect_uris, [editor, publishCallback]);
    await alice.page.getByText('App registered.', { exact: false }).waitFor();
    const copyable = JSON.parse(await alice.page.locator('#app-config-json').textContent());
    assert.deepEqual(copyable.redirect_uris, app.redirect_uris);
    const viewer = await register(alice, 'Preview', editor + 'index.html', [{ artifact_type: notes, intents: ['view'] }]);
    const query = tag => '/handoff/applications?' + new URLSearchParams({ repository: alice.repository, reference: tag });
    const matched = await (await http(files, query('notes-v1'), alice.token)).json();
    assert.equal(matched.applications.length, 2);
    assert.deepEqual(matched.applications.find(item => item.id === viewer.id).launches, { view: { intent: 'view' } });
    const restricted = await (await http(files, query('review-v1'), alice.token)).json();
    assert.deepEqual(restricted.applications.map(item => item.launches), [{ view: { intent: 'view' } }]);
    assert.deepEqual((await (await http(files, query('checklist-v1'), alice.token)).json()).applications, []);

    const page = await open(app, alice, 'edit');
    await page.locator('#connect textarea').fill(JSON.stringify({
      issuer, registry, repository: alice.repository, client_id: app.client_id,
      redirect_uri: editor, handles: app.handles
    }));
    await page.getByRole('button', { name: 'Connect', exact: true }).click();
    const access = tokenResponse(page);
    await page.getByRole('button', { name: 'Sign in to open' }).click();
    const consent = page.locator('#kc-oauth #kc-login');
    await consent.waitFor({ timeout: 5000 }).catch(() => {});
    if (await consent.isVisible()) await consent.click();
    const appToken = (await (await access).json()).access_token;
    await page.locator('#file').waitFor();
    await page.locator('#contents').fill('Revision published by the independent demo editor.');
    await page.locator('#publish input').fill('notes-v2');
    await page.getByRole('button', { name: 'Push to workspace' }).click();
    await page.getByText('Published notes-v2.', { exact: false }).waitFor();
    await alice.page.getByRole('link', { name: 'My workspace', exact: true }).click();
    await alice.page.getByRole('link', { name: 'notes-v2', exact: true }).waitFor();
    console.log('Two apps matched by artifact type. The standalone editor opened and published directly to Zot.');

    const bobCatalog = await (await http(registry, '/v2/_catalog', bob.token)).json();
    assert.deepEqual(bobCatalog.repositories, [bob.repository]);
    const aliceBase = '/v2/' + alice.repository;
    const bobBase = '/v2/' + bob.repository;
    const denied = [401, 403, 404];
    const repositoryScope = 'repository:' + alice.repository;
    const read = await authorize(alice, app, publishCallback, repositoryScope + ':pull');
    assert.equal(read.scope, repositoryScope + ':pull');
    assert.equal((await http(registry, aliceBase + '/manifests/notes-v1', read.access_token)).status, 200);
    assert(denied.includes((await http(registry, aliceBase + '/blobs/uploads/', read.access_token, 'POST')).status));
    const write = await authorize(alice, app, publishCallback, repositoryScope + ':pull ' + repositoryScope + ':push');
    const scopes = write.scope.split(/\s+/);
    assert(scopes.every(scope => scope.startsWith(repositoryScope + ':')));
    assert.deepEqual(scopes.flatMap(scope => scope.slice(repositoryScope.length + 1).split(',')).sort(), ['pull', 'push']);
    const ownManifest = await (await http(registry, aliceBase + '/manifests/notes-v2', write.access_token)).text();
    const manifestHeaders = { 'Content-Type': 'application/vnd.oci.image.manifest.v1+json' };
    assert.equal((await http(registry, aliceBase + '/manifests/repository-scope', write.access_token, 'PUT', ownManifest, manifestHeaders)).status, 201);
    assert(denied.includes((await http(registry, bobBase + '/manifests/notes-v1', write.access_token)).status));
    assert(denied.includes((await http(registry, bobBase + '/manifests/foreign', write.access_token, 'PUT', ownManifest, manifestHeaders)).status));
    const wrongUser = await authorize(bob, app, publishCallback, repositoryScope + ':pull', 400);
    assert.equal(wrongUser.error, 'invalid_grant');
    assert(denied.includes((await http(registry, aliceBase + '/tags/list', bob.token)).status));
    assert(denied.includes((await http(registry, aliceBase + '/manifests/notes-v1', bob.token)).status));
    assert(denied.includes((await http(registry, aliceBase + '/blobs/uploads/', bob.token, 'POST')).status));
    assert(denied.includes((await http(registry, bobBase + '/manifests/notes-v1', appToken)).status));
    const secretVersion = await (await http(registry, aliceBase + '/manifests/notes-v2', alice.token)).json();
    const layer = secretVersion.layers[0].digest;
    assert(denied.includes((await http(registry, aliceBase + '/blobs/' + layer, bob.token)).status));
    const mount = new URLSearchParams({ from: alice.repository, mount: layer });
    const mountResponse = await http(registry, bobBase + '/blobs/uploads/?' + mount, bob.token, 'POST');
    assert.notEqual(mountResponse.status, 201, 'cross-workspace blob mount');
    assert(denied.includes((await http(registry, bobBase + '/blobs/' + layer, bob.token)).status));
    assert.equal((await http(files, '/workspaces/' + alice.id + '/apps', bob.token)).status, 403);
    assert.equal((await http(files, '/workspaces/' + alice.id + '/apps', appToken)).status, 403);
    const request = new URLSearchParams({ service: 'zot', scope: 'repository:' + alice.repository + ':pull,push repository:' + bob.repository + ':pull,push' });
    const native = await fetch(issuer + '/protocol/docker-v2/auth?' + request, {
      headers: { Authorization: 'Basic ' + Buffer.from(bob.name + ':' + password).toString('base64') }
    });
    assert.equal(native.status, 200);
    const { token } = await native.json();
    assert(denied.includes((await http(registry, aliceBase + '/tags/list', token)).status));
    assert.equal((await http(registry, bobBase + '/tags/list', token)).status, 200);
    const bobManifest = await (await http(registry, bobBase + '/manifests/notes-v1', token)).text();
    assert.equal((await http(registry, bobBase + '/manifests/cli-v2', token, 'PUT', bobManifest,
      { 'Content-Type': 'application/vnd.oci.image.manifest.v1+json' })).status, 201);
    console.log('Workspace boundaries held at the registry for both browser and OCI-native tokens.');
    await checkProjectFiles(browser, alice);
  } finally {
    await browser.close();
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
