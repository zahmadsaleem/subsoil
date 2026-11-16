'use strict';

const assert = require('node:assert/strict');
const { randomUUID } = require('node:crypto');
const { chromium } = require('playwright');

const files = 'http://localhost:5080';
const issuer = 'http://localhost:8080/realms/subsoil';
const registry = 'http://localhost:5001';
const editor = 'http://localhost:5173/';
const password = 'Demo-pass-238!example';
const notes = 'application/vnd.subsoil.demo.notes.v1';
const review = 'application/vnd.subsoil.demo.review.v1+json';

async function verifyEmail(page, email) {
  for (let tries = 0; tries < 30; tries++) {
    const inbox = await (await fetch('http://localhost:8025/api/v1/messages')).json();
    const item = inbox.messages?.find(message => message.To.some(recipient => recipient.Address === email));
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
  assert.fail('No verification email arrived');
}

function tokenResponse(page) {
  return page.waitForResponse(reply => reply.url() === issuer + '/protocol/openid-connect/token' && reply.request().method() === 'POST');
}

async function signUp(browser, label) {
  const context = await browser.newContext({ permissions: ['clipboard-read', 'clipboard-write'] });
  const page = await context.newPage();
  const name = label + '-' + randomUUID().slice(0, 8);
  const email = name + '@example.test';
  await page.goto(files + '/explore');
  await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('link', { name: 'Register' }).click();
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
    const heading = await page.locator('h1').first().textContent().catch(() => '');
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
  const token = (await reply.json()).access_token;
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

async function main() {
  const browser = await chromium.launch({ headless: true });
  try {
    const alice = await signUp(browser, 'Alice');
    const bob = await signUp(browser, 'Bob');
    assert.notEqual(alice.repository, bob.repository);
    const app = await register(alice, 'Text editor', editor, [
      { artifact_type: notes, intents: ['view', 'edit'] }, { artifact_type: review, intents: ['view'] },
    ]);
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
    const request = new URLSearchParams({ service: 'zot', scope: 'repository:' + alice.repository + ':pull,push repository:' + bob.repository + ':pull,push' });
    const native = await fetch(issuer + '/protocol/docker-v2/auth?' + request, {
      headers: { Authorization: 'Basic ' + Buffer.from(bob.name + ':' + password).toString('base64') }
    });
    assert.equal(native.status, 200);
    const { token } = await native.json();
    assert(denied.includes((await http(registry, aliceBase + '/tags/list', token)).status));
    assert.equal((await http(registry, bobBase + '/tags/list', token)).status, 200);
    console.log('Workspace boundaries held at the registry for both browser and OCI-native tokens.');
  } finally {
    await browser.close();
  }
}

main().catch(error => { console.error(error); process.exitCode = 1; });
