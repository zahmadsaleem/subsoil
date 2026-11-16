'use strict';
(() => {
  const loginKey = 'subsoil.explorer.login';
  const $ = id => document.getElementById(id);
  const redirect = location.origin + '/explore';
  const tokens = new Map();
  const manifests = new Map();
  const pageSize = 20;
  let session, renewing, configuration, metadata, selection = 0, nextCursor;
  let view = readView(), repositoryView, tagsView;

  function status(message, error = false) {
    $('status').textContent = message;
    $('status').classList.toggle('error', error);
  }

  function signedIn(value) {
    const account = value && !session.public;
    $('browser').hidden = $('refresh').hidden = !value;
    $('sign-out').hidden = !account;
    $('sign-in').hidden = Boolean(account);
    $('account').textContent = account ? session.subject : value ? 'Public files' : '';
    $('breadcrumbs').hidden = !value;
  }

  function forget() {

    session = undefined;
    tokens.clear();
    manifests.clear();
    selection++;
    $('repositories').replaceChildren();
    $('tags').replaceChildren();
    $('artifact').hidden = true;
    $('open-with').hidden = true;
    $('open-with').replaceChildren();
    $('metadata').replaceChildren();
    $('technical-metadata').replaceChildren();
    $('files').replaceChildren();
    $('manifest').textContent = $('reference').textContent = '';
    signedIn(false);
  }

  async function run(action) {
    try { await action(); }
    catch (error) { status(error instanceof TypeError ? 'Couldn’t connect. Check your connection and try again.' : error.message || 'Couldn’t load your files. Please try again.', true); }
  }

  async function post(path, fields) {
    return fetch(path, {
      method: 'POST', body: new URLSearchParams(fields), credentials: 'omit', cache: 'no-store'
    });
  }

  async function accept(response) {
    if (!response.ok) {
      forget();
      throw Error('Please sign in again to see your files.');
    }
    const value = await response.json();

    session = { ...value, subject: 'Signed in', deadline: Date.now() + value.expires_in * 1000 };
    tokens.clear();
    signedIn(true);
  }

  async function settings() {
    if (configuration && metadata) return;
    const response = await fetch('/workflow/config', { credentials:'omit', cache:'no-store', redirect:'error' });
    if (!response.ok) throw Error('File-store settings are unavailable. Refresh and try again.');
    configuration = await response.json();
    const issuer = new URL(configuration.authorization_server);
    const discovery = new URL(issuer.pathname.replace(/\/$/, '') + '/.well-known/openid-configuration', issuer.origin);
    const reply = await fetch(discovery, { credentials:'omit', cache:'no-store', redirect:'error' });
    if (!reply.ok) throw Error('Sign-in is unavailable. Try again shortly.');
    metadata = await reply.json();
    if (metadata.issuer !== configuration.authorization_server || !metadata.code_challenge_methods_supported?.includes('S256')) throw Error('The sign-in configuration could not be verified.');
    for (const key of ['authorization_endpoint','token_endpoint']) {
      const endpoint = new URL(metadata[key]);
      if (endpoint.username || endpoint.password || endpoint.hash || endpoint.origin !== issuer.origin) throw Error('The sign-in configuration returned an unexpected address.');
    }
  }
  async function root() {
    if (session && Date.now() < session.deadline - 30000) return session.access_token;
    if (!session?.refresh_token) { forget(); throw Error('Sign in to see your folders and files.'); }
    if (!renewing) renewing = (async () => {
      await settings();
      await accept(await post(metadata.token_endpoint, { grant_type:'refresh_token', client_id:configuration.client_id, refresh_token:session.refresh_token }));
    })().finally(() => { renewing = undefined; });
    await renewing;
    return session.access_token;
  }

  async function request(path, repo, retry = true) {
    const access = await root();
    const token = access;
    const response = await fetch(path, {
      headers: { Authorization: 'Bearer ' + token }, credentials: 'omit', cache: 'no-store', redirect: 'error'
    });
    if (response.status === 401 && retry) {
      session.deadline = 0;
      tokens.clear();
      return request(path, repo, false);
    }
    if (!response.ok) throw Error(response.status === 403 ? 'These files aren’t available with your current access. Sign in or ask your project team for help.' : response.status === 401 ? 'Please sign in again to see these files.' : 'Couldn’t load these files. Please try again.');
    return response;
  }

  function readView() {
    const query = new URLSearchParams(location.search);
    return { repo: query.get('repo') || '', tag: query.get('tag') || '', query: query.get('q') || '',
      cursors: history.state?.explorer?.cursors || [query.get('last') || ''] };
  }

  function fresh(repo = '', tag = '') {
    return { repo, tag, query: '', cursors: [''] };
  }

  function viewUrl(value) {
    const query = new URLSearchParams();
    if (value.repo) query.set('repo', value.repo);
    if (value.tag) query.set('tag', value.tag);
    if (value.query) query.set('q', value.query);
    if (value.cursors.at(-1)) query.set('last', value.cursors.at(-1));
    return '/explore' + (query.size ? '?' + query : '');
  }

  function navigate(value, replace = false) {
    if (!view.repo && value.repo) repositoryView = view;
    if (view.repo && !view.tag && value.tag) tagsView = view;
    view = value;
    history[replace ? 'replaceState' : 'pushState']({ explorer: view }, '', viewUrl(view));
    return run(renderView);
  }

  function link(text, value) {
    const anchor = document.createElement('a');
    anchor.textContent = text;
    anchor.href = viewUrl(value);
    anchor.onclick = event => {
      if (event.button || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      event.preventDefault();
      navigate(value);
    };
    return anchor;
  }

  const path = repo => '/files/' + repo.split('/').map(encodeURIComponent).join('/');

  async function manifestFor(repo, tag) {
    const key = repo + ':' + tag;
    if (!manifests.has(key)) manifests.set(key, (async () => {
      const response = await request(path(repo) + '/manifests/' + encodeURIComponent(tag), repo);
      return { manifest: await response.json(), digest: response.headers.get('docker-content-digest'),
        publisher: response.headers.get('subsoil-registry-publisher'), pushed: response.headers.get('subsoil-registry-pushed-at') };
    })().catch(error => { manifests.delete(key); throw error; }));
    return manifests.get(key);
  }

  function totalSize(manifest) {
    return [manifest.config, ...(manifest.layers || [])].filter(Boolean).reduce((sum, file) => sum + file.size, 0);
  }

  async function renderView() {
    const ticket = ++selection;
    const current = view;
    const publicView = session?.public;
    const title = publicView ? 'Public files' : 'Your files';
    $('page-title').textContent = current.tag || current.repo || title;
    $('page-caption').textContent = current.tag ? 'Files in ' + current.repo : current.repo ? 'Choose a version to see its files.' : publicView ? 'Browse files available to everyone. Sign in to see files shared with your account.' : 'Find files shared with you.';
    document.title = (current.tag ? current.tag + ' · ' : '') + (current.repo || title) + ' · Subsoil';
    const crumbs = [link('Files', repositoryView || fresh())];
    if (current.repo) crumbs.push(document.createTextNode('/'), link(current.repo, tagsView?.repo === current.repo ? tagsView : fresh(current.repo)));
    if (current.tag) crumbs.push(document.createTextNode('/'), document.createTextNode(current.tag));
    $('breadcrumbs').replaceChildren(...crumbs);
    $('breadcrumbs').hidden = !current.repo;
    $('listing').hidden = Boolean(current.tag);
    $('artifact').hidden = true;
    if (current.tag) { await openArtifact(current.repo, current.tag, ticket); return; }
    manifests.clear();
    const kind = current.repo ? 'tags' : 'repositories';
    const displayKind = current.repo ? 'versions' : 'folders';
    $('search-label').textContent = 'Search ' + displayKind;
    $('search').placeholder = 'Search ' + displayKind + '…';
    $('search').value = current.query;
    $('repositories').replaceChildren();
    $('tags').replaceChildren();
    $('repositories').hidden = Boolean(current.repo);
    $('tags-table').hidden = !current.repo;
    $('empty').hidden = true;
    $('previous').disabled = $('next').disabled = true;
    $('page-summary').textContent = 'Loading…';
    status('');
    const query = new URLSearchParams({ n: pageSize, last: current.cursors.at(-1), q: current.query });
    const response = await request((current.repo ? path(current.repo) + '/tags/list' : '/files/_catalog') + '?' + query, current.repo);
    const value = await response.json();
    if (ticket !== selection) return;
    const entries = value[kind] || [];
    nextCursor = response.headers.has('link') ? entries.at(-1) : undefined;
    $('previous').disabled = current.cursors.length < 2;
    $('next').disabled = !nextCursor;
    const label = entries.length === 1 ? (current.repo ? 'version' : 'folder') : displayKind;
    $('page-summary').textContent = entries.length + ' ' + label + ' on this page';
    $('empty').hidden = entries.length > 0;
    $('empty-title').textContent = current.query ? 'No matching ' + displayKind : 'No ' + displayKind + ' yet';
    $('empty-caption').textContent = current.query ? 'Try a different name.' : publicView ? 'Public files will appear here when they’re made available.' : current.repo ? 'New versions will appear here when your team adds them.' : 'Folders shared with you will appear here.';
    if (!current.repo) {
      $('repositories').replaceChildren(...entries.map(repo => {
        const item = document.createElement('li');
        const anchor = link('', fresh(repo));
        const mark = document.createElement('span'), name = document.createElement('span'), action = document.createElement('span');
        mark.className = 'repository-mark';
        mark.textContent = '▦';
        mark.setAttribute('aria-hidden', 'true');
        name.textContent = repo;
        action.className = 'row-action';
        action.textContent = 'View versions →';
        anchor.append(mark, name, action);
        item.append(anchor);
        return item;
      }));
      return;
    }
    const queue = entries.map(tag => {
      const row = document.createElement('tr');
      const name = document.createElement('td'), published = document.createElement('td'), size = document.createElement('td');
      name.append(link(tag, fresh(current.repo, tag)));
      const publisher = document.createElement('small');
      name.append(publisher);
      published.textContent = 'Loading…';
      size.className = 'size';
      row.append(name, published, size);
      $('tags').append(row);
      return { tag, published, size, publisher };
    });
    // Only inspect this page, with bounded concurrency; never crawl other pages.
    await Promise.all(Array.from({ length: 4 }, async () => {
      while (queue.length && ticket === selection) {
        const row = queue.shift();
        try {
          const item = await manifestFor(current.repo, row.tag);
          if (ticket !== selection) return;
          row.publisher.textContent = item.publisher ? 'Added by ' + item.publisher : '';
          row.published.textContent = item.pushed ? new Date(Number(item.pushed) * 1000).toLocaleDateString() : 'Not provided';
          row.size.textContent = bytes(totalSize(item.manifest));
        } catch (error) {
          if (ticket !== selection) return;
          row.published.textContent = 'Couldn’t load details';
          row.published.title = error.message;
        }
      }
    }));
  }

  function bytes(size) {
    if (!Number.isFinite(size)) return 'Unknown';
    if (size < 1024) return size + ' B';
    if (size < 1024 ** 2) return (size / 1024).toFixed(1) + ' KB';
    if (size < 1024 ** 3) return (size / 1024 ** 2).toFixed(1) + ' MB';
    return (size / 1024 ** 3).toFixed(1) + ' GB';
  }

  function fileType(file) {
    const name = file.annotations?.['org.opencontainers.image.title'] || '';
    const extension = name.toLowerCase().split('.').at(-1);
    const types = { ifc: 'Building model', glb: '3D model', gltf: '3D model', pdf: 'PDF document',
      json: 'Data file', parquet: 'Data table', csv: 'Spreadsheet', xls: 'Spreadsheet', xlsx: 'Spreadsheet',
      dwg: 'Drawing', dxf: 'Drawing', zip: 'ZIP archive', txt: 'Text document',
      png: 'Image', jpg: 'Image', jpeg: 'Image', gif: 'Image', webp: 'Image', svg: 'Image' };
    if (Object.hasOwn(types, extension)) return types[extension];
    const mediaType = file.mediaType || '';
    if (mediaType.startsWith('image/')) return 'Image';
    if (mediaType.startsWith('model/')) return '3D model';
    if (mediaType === 'application/pdf') return 'PDF document';
    if (mediaType === 'application/json' || mediaType.endsWith('+json')) return 'Data file';
    if (mediaType === 'application/vnd.apache.parquet') return 'Data table';
    if (mediaType.startsWith('text/')) return 'Text document';
    return 'File';
  }

  function metadataRows(entries) {
    return entries.flatMap(([name, value]) => {
      const dt = document.createElement('dt'), dd = document.createElement('dd');
      dt.textContent = name;
      dd.textContent = value || 'Not provided';
      return [dt, dd];
    });
  }

  async function openArtifact(repo, tag, ticket) {
    $('artifact').hidden = true;
    $('open-with').hidden = true;
    $('open-with').replaceChildren();
    status('Loading files…');
    const { manifest, digest, pushed, publisher } = await manifestFor(repo, tag);
    if (ticket !== selection) return;
    const files = [manifest.config, ...(manifest.layers || [])].filter(Boolean);
    const metadata = [
      ['Version', tag],
      ['Added by', publisher || 'Not provided'],
      ['Added', pushed ? new Date(Number(pushed) * 1000).toLocaleString() : 'Not provided'],
      ['Total size', bytes(totalSize(manifest))]
    ];
    $('artifact-title').textContent = manifest.annotations?.['org.opencontainers.image.title'] || tag;
    $('metadata').replaceChildren(...metadataRows(metadata));
    $('technical-metadata').replaceChildren(...metadataRows([
      ['Storage format', manifest.artifactType || manifest.config?.mediaType || manifest.mediaType],
      ['Content ID', digest]
    ]));
    const connection = await (await fetch('/workflow/config')).json();
    $('reference').textContent = new URL(connection.registry).host + '/' + repo + (digest ? '@' + digest : ':' + tag);
    $('copy').textContent = 'Copy link';
    $('share-caption').textContent = session.public ? 'Anyone can view this public version.' : 'Share this version with someone who has access.';
    $('copy-reference').textContent = 'Copy technical reference';
    $('file-count').textContent = files.length;
    $('files').replaceChildren(...files.map((file, index) => {
      const row = document.createElement('tr');
      const name = document.createElement('td'), type = document.createElement('td'), size = document.createElement('td'), action = document.createElement('td');
      const title = file.annotations?.['org.opencontainers.image.title'] || (index === 0 ? 'File information' : 'File ' + index);
      name.textContent = title;
      type.textContent = fileType(file);
      size.textContent = bytes(file.size);
      size.className = 'size';
      const download = document.createElement('button');
      download.type = 'button';
      download.textContent = 'Download';
      download.setAttribute('aria-label', 'Download ' + title);
      download.onclick = () => run(async () => {
        download.disabled = true;
        try {
          const response = await request(path(repo) + '/blobs/' + encodeURIComponent(file.digest), repo);
          const content = await response.arrayBuffer();
          if (content.byteLength !== file.size) throw Error('This file’s size doesn’t match the published version. Refresh and try again.');
          const hash = new Uint8Array(await crypto.subtle.digest('SHA-256', content));
          const fingerprint = 'sha256:' + [...hash].map(value => value.toString(16).padStart(2, '0')).join('');
          if (fingerprint !== file.digest) throw Error('This file doesn’t match the published version. Refresh and try again.');
          const url = URL.createObjectURL(new Blob([content], { type: file.mediaType || 'application/octet-stream' }));
          const anchor = document.createElement('a');
          anchor.href = url;
          anchor.download = title.split('/').at(-1);
          anchor.click();
          setTimeout(() => URL.revokeObjectURL(url), 1000);
        } finally { download.disabled = false; }
      });
      action.append(download);
      row.append(name, type, size, action);
      return row;
    }));
    $('manifest').textContent = JSON.stringify(manifest, null, 2);
    $('artifact').hidden = false;
    status('');
    if (!session.public) window.SubsoilHandoffUI($('open-with'), root, repo, digest || tag, () => ticket === selection)
      .catch(error => { if (ticket === selection) status(error.message, true); });
  }

  const base64 = bytes => btoa(String.fromCharCode(...bytes)).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
  const random = () => base64(crypto.getRandomValues(new Uint8Array(32)));

  $('sign-in').onclick = () => run(async () => {
    if (!crypto.subtle) throw Error('Open the secure address your team gave you, then sign in again.');
    await settings();
    const verifier = random(), state = random();
    const challenge = base64(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(verifier))));
    sessionStorage.setItem(loginKey, JSON.stringify({ verifier, state, created:Date.now(), view, issuer:configuration.authorization_server }));
    const url = new URL(metadata.authorization_endpoint);
    url.search = new URLSearchParams({ response_type:'code', client_id:configuration.client_id, redirect_uri:redirect,
      scope:configuration.scopes.join(' '), resource:configuration.registry, state, code_challenge:challenge, code_challenge_method:'S256' });
    location.assign(url);
  });
  $('sign-out').onclick = () => run(async () => {
    if (renewing) await renewing;
    forget();
    status('Disconnected from these files. Your sign-in provider manages your account session.');
  });
  $('refresh').onclick = () => { manifests.clear(); if (session?.public) { session.deadline = 0; tokens.clear(); } run(async () => { await root(); await renderView(); }); };
  $('repositories-nav').onclick = event => { event.preventDefault(); navigate(fresh()); };
  $('search-form').onsubmit = event => {
    event.preventDefault();
    navigate({ ...view, query: $('search').value.trim(), cursors: [''] });
  };
  $('previous').onclick = () => navigate({ ...view, cursors: view.cursors.slice(0, -1) });
  $('next').onclick = () => { if (nextCursor) navigate({ ...view, cursors: [...view.cursors, nextCursor] }); };
  window.addEventListener('popstate', () => { view = readView(); if (session) run(renderView); });
  $('copy').onclick = () => run(async () => {
    try {
      await navigator.clipboard.writeText(new URL(viewUrl(fresh(view.repo, view.tag)), location.origin).href);
      $('copy').textContent = 'Link copied';
    } catch { throw Error('Couldn’t copy the link. Copy the address from your browser instead.'); }
  });
  $('copy-reference').onclick = () => run(async () => {
    try {
      await navigator.clipboard.writeText($('reference').textContent);
      $('copy-reference').textContent = 'Reference copied';
    } catch { throw Error('Couldn’t copy this reference. Select and copy the text instead.'); }
  });

  run(async () => {
    const query = new URLSearchParams(location.search);
    if (query.has('code') || query.has('error')) {
      history.replaceState(null, '', '/explore');
      const pending = JSON.parse(sessionStorage.getItem(loginKey) || 'null');
      sessionStorage.removeItem(loginKey);
      if (['code','state','iss','error'].some(key => query.getAll(key).length > 1) || !pending || pending.state !== query.get('state') || Date.now() - pending.created > 300000) {
        throw Error('Your sign-in link has expired. Please sign in again.');
      }
      if (query.has('error')) { status('Sign-in cancelled.'); return; }
      await settings();
      if (pending.issuer !== configuration.authorization_server || query.getAll('iss').length !== 1 || query.get('iss') !== pending.issuer) throw Error('The sign-in response came from another issuer.');
      await accept(await post(metadata.token_endpoint, { grant_type: 'authorization_code', client_id: configuration.client_id,
        redirect_uri: redirect, code: query.get('code'), code_verifier: pending.verifier, resource:configuration.registry }));
      view = pending.view || fresh();
    } else { signedIn(false); status('Sign in to see your folders and files.'); return; }
    history.replaceState({ explorer: view }, '', viewUrl(view));
    await renderView();
  });
})();
