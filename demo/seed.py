"""Seed the sample IFC revisions in Zot, leaving existing versions in place."""
import hashlib, json, os, pathlib, time, urllib.error, urllib.parse, urllib.request
REGISTRY=os.getenv('REGISTRY_URL','http://registry:8080')
ISSUER=os.getenv('KEYCLOAK_URL','http://keycloak:8080/realms/subsoil')
PUBLIC_REGISTRY=os.getenv('REGISTRY_PUBLIC','http://localhost:5001')
SEED_SECRET=os.getenv('SEED_SECRET','demo-seed-secret')
def request(url, method='GET', body=None, token=None, content_type=None):
    headers={}
    if token: headers['Authorization']='Bearer '+token
    if content_type: headers['Content-Type']=content_type
    return urllib.request.urlopen(urllib.request.Request(url, data=body, method=method, headers=headers), timeout=10)
def sha(data): return 'sha256:'+hashlib.sha256(data).hexdigest()
def login():
    fields=urllib.parse.urlencode({'grant_type':'client_credentials','client_id':'demo-seed','client_secret':SEED_SECRET,'scope':'repository:project/model:pull repository:project/model:push','resource':PUBLIC_REGISTRY}).encode()
    with request(ISSUER+'/protocol/openid-connect/token', 'POST',fields, content_type='application/x-www-form-urlencoded') as response: return json.load(response)['access_token']
def upload(data, token):
    with request(REGISTRY+'/v2/project/model/blobs/uploads/', 'POST', b'', token) as response: location=response.headers['Location']
    parsed=urllib.parse.urlsplit(urllib.parse.urljoin(REGISTRY,location))
    # The in-network script uses the exact upload path at its configured public registry gateway.
    location=REGISTRY+parsed.path+('?' + parsed.query if parsed.query else '')
    location+=('&' if parsed.query else '?')+urllib.parse.urlencode({'digest':sha(data)})
    with request(location,'PUT',data,token,'application/octet-stream') as response: assert response.status==201
for attempt in range(120):
    try:
        token=login()
        with request(REGISTRY+'/v2/',token=token): pass
        break
    except (urllib.error.URLError, OSError) as error:
        if attempt==119: raise
        time.sleep(1)


def seed_artifact(tag, artifact_type, files, annotations):
    target = REGISTRY + '/v2/project/model/manifests/' + tag
    try:
        with request(target, token=token):
            print(f'Keeping the existing project/model:{tag} version.', flush=True)
            return
    except urllib.error.HTTPError as error:
        if error.code != 404:
            raise
    config = b'{}'
    upload(config, token)
    layers = []
    for data, media_type, file_annotations in files:
        upload(data, token)
        layers.append({
            'mediaType': media_type, 'digest': sha(data), 'size': len(data),
            'annotations': file_annotations,
        })
    manifest = json.dumps({
        'schemaVersion': 2,
        'mediaType': 'application/vnd.oci.image.manifest.v1+json',
        'artifactType': artifact_type,
        'config': {'mediaType': 'application/vnd.oci.empty.v1+json', 'digest': sha(config), 'size': len(config)},
        'layers': layers,
        'annotations': annotations,
    }, separators=(',', ':')).encode()
    with request(target, 'PUT', manifest, token, 'application/vnd.oci.image.manifest.v1+json') as response:
        assert response.status == 201
    print(f'Published project/model:{tag} to Zot.', flush=True)


def seed_revision(tag, source, version_label=None):
    annotations = {'org.opencontainers.image.title': 'Example building model'}
    if version_label:
        annotations['org.opencontainers.image.version'] = version_label
    seed_artifact(tag, 'application/vnd.subsoil.file.v1', [
        (source, 'application/x-step', {'org.opencontainers.image.title': 'model.ifc'}),
    ], annotations)


root = pathlib.Path('/demo')
source = (root / 'model.ifc').read_bytes()
revised = source.replace(b'((5.,0.,0.))', b'((6.,0.,0.))', 1)
assert revised != source
seed_revision('issued', source)
seed_revision('revision-01', source, 'revision-01')
seed_revision('revision-02', revised, 'revision-02')

package = json.loads((root / 'work-package/releases.json').read_text())
for release in package['releases']:
    files = [
        ((root / file['source']).read_bytes(), file['media_type'], {
            'org.opencontainers.image.title': file['name'],
            'dev.subsoil.file.role': file['role'],
        })
        for file in release['files']
    ]
    seed_artifact(release['tag'], package['artifact_type'], files, {
        'org.opencontainers.image.title': package['title'],
        'dev.subsoil.release.by': release['released_by'],
        'dev.subsoil.release.at': release['released_at'],
    })
