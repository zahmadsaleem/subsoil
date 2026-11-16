'use strict';

window.SubsoilWorkspace = (() => {
  const element = id => document.getElementById(id);
  const panel = element('workspace-apps');
  const form = element('register-app');
  const message = element('app-status');
  let disconnect;

  async function connect(credential) {
    disconnect?.();
    const controller = new AbortController();
    disconnect = () => {
      controller.abort();
      panel.hidden = true;
      element('app-config').hidden = true;
      element('app-config-json').textContent = '';
      element('registered-apps').replaceChildren();
    };
    async function call(method, path, data) {
      const response = await fetch(path, {
        method, credentials: 'omit', cache: 'no-store', redirect: 'error', signal: controller.signal,
        headers: { Authorization: 'Bearer ' + await credential(), 'Content-Type': 'application/json' },
        body: data && JSON.stringify(data)
      });
      if (response.status === 204) return;
      const result = await response.json();
      if (!response.ok) throw Error(result.message || 'Couldn’t manage your workspace apps. Try again.');
      return result;
    }
    const workspace = await call('POST', '/workspace');
    if (!workspace || controller.signal.aborted) return;
    const endpoint = '/workspaces/' + encodeURIComponent(workspace.id) + '/apps';
    const showConfig = app => {
      const config = {
        issuer: app.issuer, registry: app.registry, repository: app.repository,
        client_id: app.client_id, redirect_uri: app.redirect_uris[0], handles: app.handles
      };
      const json = JSON.stringify(config, null, 2);
      element('app-config-json').textContent = json;
      element('app-config').hidden = false;
      element('app-config').open = true;
      return json;
    };
    async function act(action) {
      message.textContent = '';
      try { await action(); } catch (error) {
        if (!controller.signal.aborted) message.textContent = error.message;
      }
    }
    function button(label, action) {
      const button = document.createElement('button');
      button.textContent = label;
      button.onclick = () => act(action);
      return button;
    }
    async function list() {
      const registrations = await call('GET', endpoint);
      element('registered-apps').replaceChildren(...registrations.map(app => {
        const item = document.createElement('li'), name = document.createElement('strong');
        name.textContent = app.name;
        item.append(name, button('Copy app config', async () => {
          await navigator.clipboard.writeText(showConfig(app));
          message.textContent = 'App configuration copied.';
        }), button('Remove', async () => {
          await call('DELETE', endpoint + '/' + encodeURIComponent(app.id));
          element('app-config').hidden = true;
          await list();
          message.textContent = 'App removed.';
          window.dispatchEvent(new Event('subsoil-apps-changed'));
        }));
        return item;
      }));
    }
    form.onsubmit = event => {
      event.preventDefault();
      act(async () => {
        const fields = new FormData(form), handles = new Map();
        for (const intent of ['view', 'edit']) {
          for (const type of fields.get(intent + '_types').split(',').map(value => value.trim()).filter(Boolean)) {
            if (!handles.has(type)) handles.set(type, new Set());
            handles.get(type).add(intent);
          }
        }
        const app = await call('POST', endpoint, {
          name: fields.get('app_name'), redirect_uris: [fields.get('callback_url')],
          launch_url: fields.get('launch_url'),
          handles: [...handles].map(([artifact_type, intents]) => ({ artifact_type, intents: [...intents] }))
        });
        await list();
        showConfig(app);
        message.textContent = 'App registered. Copy its configuration into your app.';
        window.dispatchEvent(new Event('subsoil-apps-changed'));
      });
    };
    await list();
    panel.hidden = false;
    return workspace.repository;
  }
  return { connect, disconnect: () => disconnect?.() };
})();
