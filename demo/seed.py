"""Publish a real IFC artifact through Keycloak and Zot; safe to rerun with identical bytes."""
import hashlib, json, os, pathlib, time, urllib.error, urllib.parse, urllib.request
REGISTRY=os.getenv('REGISTRY_URL','http://registry:8080')
ISSUER=os.getenv('KEYCLOAK_URL','http://keycloak:8080/realms/subsoil')
def request(url, method='GET', body=None, token=None, content_type=None):
    headers={}
    if token: headers['Authorization']='Bearer '+token
    if content_type: headers['Content-Type']=content_type
    return urllib.request.urlopen(urllib.request.Request(url, data=body, method=method, headers=headers), timeout=10)
def sha(data): return 'sha256:'+hashlib.sha256(data).hexdigest()
def login():
    fields=urllib.parse.urlencode({'grant_type':'client_credentials','client_id':'demo-seed','client_secret':'demo-seed-secret','scope':'repository:project/model:pull repository:project/model:push','resource':'http://localhost:5001'}).encode()
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
source=pathlib.Path('/demo/model.ifc').read_bytes(); config=b'{}'
for data in [config,source]: upload(data,token)
manifest=json.dumps({'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json','artifactType':'application/vnd.ifc-lite.ifc.v1',
 'config':{'mediaType':'application/vnd.oci.empty.v1+json','digest':sha(config),'size':len(config)},
 'layers':[{'mediaType':'application/x-step','digest':sha(source),'size':len(source),'annotations':{'org.opencontainers.image.title':'model.ifc'}}],
 'annotations':{'org.opencontainers.image.title':'Example building model'}},separators=(',',':')).encode()
with request(REGISTRY+'/v2/project/model/manifests/issued','PUT',manifest,token,'application/vnd.oci.image.manifest.v1+json') as response: assert response.status==201
print('Published project/model:issued to Zot. Open http://localhost:5080/explore and sign in as writer or reader.',flush=True)
