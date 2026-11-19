'use strict';

const configKey = 'subsoil.starter.config:' + location.pathname;
const pendingKey = 'subsoil.starter.login:' + location.pathname;
const manifestType = 'application/vnd.oci.image.manifest.v1+json';
const maxFileSize = 4 * 1024 * 1024;
const packageType = 'application/vnd.subsoil.demo.work-package.v1';
const types = new Map([
  ['application/vnd.subsoil.demo.notes.v1', new Set(['view', 'edit'])],
  ['application/vnd.subsoil.demo.checklist.v1+json', new Set(['view', 'edit'])],
  ['application/vnd.subsoil.demo.review.v1+json', new Set(['view'])],
]);
const $ = id => document.getElementById(id);
const encoder = new TextEncoder(), decoder = new TextDecoder('utf-8', { fatal: true });
let config, handoff, metadata, session, opened, layer, previewURL;

function safeAddress(value) {
  const address = new URL(value);
  if (address.username || address.password || address.search || address.hash ||
      (address.protocol !== 'https:' &&
       !(address.protocol === 'http:' && address.hostname === 'localhost'))) {
    throw Error('Use an HTTPS address or the local demo address.');
  }
  return address;
}

function readConfig(value) {
  if (safeAddress(value.registry).origin !== value.registry ||
      new URL(value.redirect_uri).href !== location.origin + location.pathname ||
      !/^workspaces\/[a-f0-9-]{36}\/scratch$/.test(value.repository) ||
      typeof value.client_id !== 'string' || !Array.isArray(value.handles)) {
    throw Error('Paste the configuration for this app from Workspace apps.');
  }
  safeAddress(value.issuer);
  return value;
}

const base64 = bytes => btoa(String.fromCharCode(...bytes)).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
const random = () => base64(crypto.getRandomValues(new Uint8Array(32)));
async function sha(bytes) {
  const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
  return 'sha256:' + [...hash].map(byte => byte.toString(16).padStart(2, '0')).join('');
}

function readHandoff(hash) {
  const value = new URLSearchParams(hash.slice(1)).get('handoff');
  if (!value || value.length > 4096) throw Error('Choose Open with in Subsoil and copy the app link.');
  const bytes = Uint8Array.from(atob(value.replaceAll('-', '+').replaceAll('_', '/')), char => char.charCodeAt(0));
  const data = JSON.parse(decoder.decode(bytes));
  if (data.protocol !== 'oci-handoff/1' || !['view', 'edit'].includes(data.intent) ||
      !/^sha256:[a-f0-9]{64}$/.test(data.source?.digest)) throw Error('The file link is invalid.');
  return data;
}

function readLaunch(parameters, hash) {
  const values = parameters.getAll('layer');
  if (values.length > 1 || (values.length === 1 && !/^(0|[1-9][0-9]{0,8})$/.test(values[0]))) {
    throw Error('The file selection is invalid. Open the file again from Subsoil.');
  }
  handoff = readHandoff(hash);
  layer = values.length ? Number(values[0]) : undefined;
}

function validateHandoff(data) {
  if (!config || data.source.registry !== config.registry || data.source.repository !== config.repository) {
    throw Error('Choose a file from the workspace in this app’s configuration.');
  }
  return data;
}

async function discover() {
  if (metadata) return;
  const response = await fetch(config.issuer + '/.well-known/openid-configuration', {
    credentials: 'omit', cache: 'no-store', redirect: 'error'
  });
  if (!response.ok) throw Error('Couldn’t reach the sign-in provider.');
  const found = await response.json(), origin = new URL(config.issuer).origin;
  if (found.issuer !== config.issuer || !found.code_challenge_methods_supported?.includes('S256') ||
      [found.authorization_endpoint, found.token_endpoint].some(endpoint => safeAddress(endpoint).origin !== origin)) {
    throw Error('Couldn’t verify the sign-in provider.');
  }
  metadata = found;
}

async function exchange(fields) {
  await discover();
  const response = await fetch(metadata.token_endpoint, {
    method: 'POST', body: new URLSearchParams({ client_id: config.client_id, resource: config.registry, ...fields }),
    credentials: 'omit', cache: 'no-store', redirect: 'error'
  });
  if (!response.ok) throw Error('Sign in again to access this workspace.');
  const result = await response.json();
  if (!result.access_token || !String(result.scope).split(' ').includes('workspace:read')) {
    throw Error('This account cannot read the app’s registered workspace.');
  }
  session = { ...result, expires: Date.now() + result.expires_in * 1000 };
}

async function signIn() {
  validateHandoff(handoff);
  await discover();
  const state = random(), verifier = random();
  const challenge = base64(new Uint8Array(await crypto.subtle.digest('SHA-256', encoder.encode(verifier))));
  sessionStorage.setItem(pendingKey, JSON.stringify({ state, verifier, handoff, layer, issuer: config.issuer, client: config.client_id, started: Date.now() }));
  const endpoint = new URL(metadata.authorization_endpoint);
  endpoint.search = new URLSearchParams({
    response_type: 'code', client_id: config.client_id, redirect_uri: config.redirect_uri, resource: config.registry,
    scope: handoff.intent === 'edit' ? 'openid workspace:read workspace:write' : 'openid workspace:read',
    code_challenge_method: 'S256', code_challenge: challenge, state
  });
  location.assign(endpoint);
}

async function registry(path, method = 'GET', body, contentType) {
  if (!session) throw Error('Sign in to read the file.');
  if (Date.now() >= session.expires - 20000) {
    if (!session.refresh_token) throw Error('Sign in again to continue.');
    await exchange({ grant_type: 'refresh_token', refresh_token: session.refresh_token });
  }
  const headers = { Authorization: 'Bearer ' + session.access_token, Accept: manifestType };
  if (contentType) headers['Content-Type'] = contentType;
  return fetch(config.registry + path, { method, body, headers, credentials: 'omit', cache: 'no-store', redirect: 'error' });
}

async function checked(response, expected) {
  if (response.status === 413) throw Error('This upload exceeds the demo workspace limits.');
  if (!response.ok) throw Error('The workspace rejected the request. Check access and try again.');
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (await sha(bytes) !== expected) throw Error('The downloaded contents failed their integrity check.');
  return bytes;
}

async function openFile() {
  const base = '/v2/' + config.repository;
  const pointer = handoff.source.digest;
  const manifest = JSON.parse(decoder.decode(await checked(await registry(base + '/manifests/' + pointer), pointer)));
  const type = manifest.artifactType;
  const capability = config.handles.some(handle => handle.artifact_type === type && handle.intents.includes(handoff.intent));
  if (layer !== undefined || (type === packageType && capability)) {
    await openPackage(base, manifest, layer);
    return;
  }
  const file = manifest.layers?.[0];
  if (!capability || !types.get(type)?.has(handoff.intent) || manifest.layers?.length !== 1 ||
      !/^sha256:[a-f0-9]{64}$/.test(file?.digest) || file.size > maxFileSize ||
      !['text/plain', 'application/json'].includes(file.mediaType)) throw Error('This app cannot open the selected artifact type and action.');
  $('contents').value = decoder.decode(await checked(await registry(base + '/blobs/' + file.digest), file.digest));
  $('filename').textContent = file.annotations?.['org.opencontainers.image.title'] || 'File contents';
  const editable = handoff.intent === 'edit' && String(session.scope).split(' ').includes('workspace:write');
  $('contents').readOnly = !editable;
  $('file').hidden = false;
  $('publish').hidden = !editable;
  $('signin').hidden = true;
  opened = { manifest, file, editable };
  $('status').textContent = 'File opened.';
}

function memberName(file) {
  return (file.annotations?.['org.opencontainers.image.title'] || 'File').split(/[\\/]/).pop();
}

async function viewMember(base, file) {
  if (!/^sha256:[a-f0-9]{64}$/.test(file?.digest) ||
      !Number.isSafeInteger(file.size) || file.size < 0 || file.size > maxFileSize) {
    throw Error('This file is unavailable or exceeds the workspace app’s size limit.');
  }
  const data = await checked(await registry(base + '/blobs/' + file.digest), file.digest);
  if (data.byteLength !== file.size) throw Error('The downloaded file has an unexpected size.');
  if (previewURL) URL.revokeObjectURL(previewURL);
  const renderable = ['application/pdf', 'image/png', 'image/jpeg'].includes(file.mediaType);
  previewURL = URL.createObjectURL(new Blob([data], { type: renderable ? file.mediaType : 'application/octet-stream' }));
  const name = memberName(file);
  $('member-title').textContent = name;
  $('member-download').href = previewURL;
  $('member-download').download = name;
  $('preview').replaceChildren();
  if (file.mediaType === 'application/pdf') {
    const frame = document.createElement('iframe');
    frame.title = name;
    frame.src = previewURL;
    $('preview').append(frame);
  } else if (file.mediaType === 'image/png' || file.mediaType === 'image/jpeg') {
    const image = document.createElement('img');
    image.alt = name;
    image.src = previewURL;
    $('preview').append(image);
  } else if (file.mediaType === 'text/plain') {
    const text = document.createElement('pre');
    text.textContent = decoder.decode(data);
    $('preview').append(text);
  }
  $('member').hidden = false;
  $('status').textContent = 'File opened.';
}

async function openPackage(base, manifest, selected) {
  if (handoff.intent !== 'view' || !Array.isArray(manifest.layers)) throw Error('This workspace app can view a release.');
  const accepts = file => (config.file_handles || []).some(handle =>
    handle.media_type === file.mediaType && handle.intents.includes(handoff.intent));
  const file = selected === undefined ? undefined : manifest.layers[selected];
  if (selected !== undefined && (!file || !accepts(file))) {
    throw Error('This app cannot open the selected file type.');
  }
  $('file').hidden = true;
  $('member').hidden = true;
  $('package-title').textContent = manifest.annotations?.['org.opencontainers.image.title'] || 'Release files';
  $('package-files').replaceChildren(...manifest.layers.flatMap(entry => {
    if (selected !== undefined && !accepts(entry)) return [];
    const item = document.createElement('li');
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = memberName(entry);
    button.onclick = () => run(() => viewMember(base, entry));
    item.append(button);
    return [item];
  }));
  $('package').hidden = false;
  $('signin').hidden = true;
  $('status').textContent = 'Release opened.';
  if (file) await viewMember(base, file);
}

async function upload(base, bytes) {
  const digest = await sha(bytes);
  const uploads = base + '/blobs/uploads/';
  const started = await registry(uploads, 'POST');
  if (started.status !== 202 || !started.headers.has('Location')) throw Error('Couldn’t start the upload. Check your workspace access.');
  const target = new URL(started.headers.get('Location'), config.registry);
  if (!target.pathname.startsWith(uploads)) throw Error('The registry returned an unexpected upload address.');
  target.searchParams.set('digest', digest);
  const stored = await registry(target.pathname + target.search, 'PUT', bytes, 'application/octet-stream');
  if (stored.status !== 201) throw Error(stored.status === 413 ? 'The file exceeds the demo upload limit.' : 'Couldn’t upload this file. Try again.');
  return { digest, size: bytes.length };
}

async function publish(tag) {
  if (!opened?.editable || !/^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$/.test(tag)) throw Error('Enter a valid new version name.');
  const base = '/v2/' + config.repository, path = base + '/manifests/' + tag;
  async function ensureNew() {
    const current = await registry(path, 'HEAD');
    if (current.ok) throw Error('That version already exists. Choose another name.');
    if (current.status !== 404) throw Error('Couldn’t check the version name. Check your access.');
  }
  const contents = $('contents').value, data = encoder.encode(contents);
  if (data.length > maxFileSize) throw Error('The file exceeds the 4 MiB demo upload limit.');
  if (opened.file.mediaType === 'application/json') {
    try { JSON.parse(contents); } catch { throw Error('Fix the JSON syntax before publishing.'); }
  }
  await ensureNew();
  const configDescriptor = await upload(base, encoder.encode('{}'));
  const layerDescriptor = await upload(base, data);
  const manifest = {
    schemaVersion: 2, mediaType: manifestType, artifactType: opened.manifest.artifactType,
    config: { mediaType: 'application/vnd.oci.empty.v1+json', ...configDescriptor },
    layers: [{ mediaType: opened.file.mediaType, annotations: opened.file.annotations, ...layerDescriptor }],
  };
  await ensureNew();
  const saved = await registry(path, 'PUT', JSON.stringify(manifest), manifestType);
  if (saved.status !== 201) throw Error(saved.status === 413 ? 'The workspace storage limit was reached.' : 'Couldn’t publish this version. Try again.');
  $('status').textContent = 'Published ' + tag + '. Refresh your workspace in Subsoil to see it.';
}

async function run(action) {
  try { await action(); } catch (error) {
    $('status').textContent = error instanceof TypeError ? 'Couldn’t connect. Check the app configuration and try again.' : error.message;
  }
}

function ready() {
  $('signin').hidden = true;
  if (handoff && config) {
    validateHandoff(handoff);
    $('signin').hidden = false;
    $('status').textContent = 'Sign in to open the selected version.';
  } else if (handoff) {
    $('settings').open = true;
    $('status').textContent = 'Connect your app to open this file.';
  }
}

$('connect').onsubmit = event => {
  event.preventDefault();
  run(async () => {
    config = readConfig(JSON.parse(new FormData(event.currentTarget).get('settings')));
    localStorage.setItem(configKey, JSON.stringify(config));
    metadata = session = opened = undefined;
    $('file').hidden = true;
    $('package').hidden = true;
    $('settings').open = false;
    $('status').textContent = 'App connected. Open a file from Subsoil.';
    ready();
  });
};
$('open').onsubmit = event => {
  event.preventDefault();
  run(async () => {
    const address = new URL(new FormData(event.currentTarget).get('link'), location.href);
    readLaunch(address.searchParams, address.hash);
    ready();
  });
};
$('signin').onclick = () => run(signIn);
$('publish').onsubmit = event => {
  event.preventDefault();
  const button = event.currentTarget.querySelector('button');
  const tag = new FormData(event.currentTarget).get('tag').trim();
  button.disabled = true;
  run(() => publish(tag)).finally(() => { button.disabled = false; });
};

run(async () => {
  const parameters = new URLSearchParams(location.search);
  const hash = location.hash;
  history.replaceState(null, '', location.pathname);
  const saved = localStorage.getItem(configKey);
  if (saved) config = readConfig(JSON.parse(saved));
  if (parameters.has('code') || parameters.has('error')) {
    const pending = JSON.parse(sessionStorage.getItem(pendingKey) || 'null');
    sessionStorage.removeItem(pendingKey);
    if (parameters.has('error')) throw Error('Sign-in was cancelled. Reopen the file from Subsoil.');
    if (['code', 'state', 'iss'].some(key => parameters.getAll(key).length !== 1) ||
        !pending || !config || Date.now() - pending.started > 300000 ||
        pending.state !== parameters.get('state') || pending.issuer !== parameters.get('iss') ||
        pending.issuer !== config.issuer || pending.client !== config.client_id) throw Error('The sign-in link expired. Reopen the file from Subsoil.');
    handoff = validateHandoff(pending.handoff);
    layer = pending.layer;
    await exchange({ grant_type: 'authorization_code', code: parameters.get('code'), code_verifier: pending.verifier,
      redirect_uri: config.redirect_uri });
    await openFile();
  } else {
    if (hash) readLaunch(parameters, hash);
    ready();
  }
});
