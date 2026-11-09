"""End-to-end contract checks against the Compose Keycloak, gateway, Zot and Subsoil services."""
import base64, hashlib, html, http.cookiejar, json, os, pathlib, secrets, time, urllib.error, urllib.parse, urllib.request
REGISTRY=os.getenv('REGISTRY_URL','http://localhost:5001')
SUBSOIL=os.getenv('SUBSOIL_URL','http://localhost:5080')
ISSUER=os.getenv('ISSUER_URL','http://localhost:8080/realms/subsoil')
PUBLIC_ISSUER=os.getenv('ISSUER_PUBLIC',ISSUER)
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self,*args): return None

def response(url,method='GET',data=None,token=None,content_type=None,opener=None):
    headers={}
    if token: headers['Authorization']='Bearer '+token
    if content_type: headers['Content-Type']=content_type
    req=urllib.request.Request(url,data=data,headers=headers,method=method)
    try: return (opener or urllib.request.build_opener()).open(req,timeout=15)
    except urllib.error.HTTPError as error: return error

def oauth(username,scope,client='ifc-lite',callback='http://localhost:3000/open-artifact/callback'):
    verifier=secrets.token_urlsafe(32);state=secrets.token_urlsafe(32)
    challenge=base64.urlsafe_b64encode(hashlib.sha256(verifier.encode()).digest()).decode().rstrip('=')
    query=urllib.parse.urlencode({'client_id':client,'redirect_uri':callback,'response_type':'code','scope':scope,'resource':'http://localhost:5001','state':state,'code_challenge_method':'S256','code_challenge':challenge})
    class DemoCookiePolicy(http.cookiejar.DefaultCookiePolicy):
        def return_ok_secure(self,cookie,request):
            # Browser localhost development treats this HTTP origin as a secure context.
            if urllib.parse.urlsplit(request.full_url).netloc==urllib.parse.urlsplit(ISSUER).netloc: return True
            return super().return_ok_secure(cookie,request)
    jar=http.cookiejar.CookieJar(policy=DemoCookiePolicy());opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar),NoRedirect())
    page=response(ISSUER+'/protocol/openid-connect/auth?'+query,opener=opener)
    assert page.status==200, ('login',page.status,page.read().decode()[:200])
    from html.parser import HTMLParser
    class Form(HTMLParser):
        action=None
        def handle_starttag(self,tag,attrs):
            attrs=dict(attrs)
            if tag=='form' and attrs.get('id')=='kc-form-login': self.action=attrs.get('action')
    form=Form();form.feed(page.read().decode());assert form.action,'Keycloak login form'
    fields=urllib.parse.urlencode({'username':username,'password':username+'-demo-password','credentialId':''}).encode()
    action=html.unescape(form.action).replace(PUBLIC_ISSUER,ISSUER,1)
    logged=response(action,'POST',fields,content_type='application/x-www-form-urlencoded',opener=opener)
    assert logged.status in (302,303),('login result',logged.status,logged.read().decode()[:500])
    location=logged.headers['Location'];params=urllib.parse.parse_qs(urllib.parse.urlsplit(location).query)
    assert params.get('state')==[state] and params.get('iss')==[PUBLIC_ISSUER],params
    assert 'code' in params, params
    fields=urllib.parse.urlencode({'grant_type':'authorization_code','client_id':client,'redirect_uri':callback,'code':params['code'][0],'code_verifier':verifier,'resource':'http://localhost:5001'}).encode()
    reply=response(ISSUER+'/protocol/openid-connect/token','POST',fields,content_type='application/x-www-form-urlencoded')
    assert reply.status==200,('token',reply.status,reply.read().decode())
    token=json.load(reply)
    replay=response(ISSUER+'/protocol/openid-connect/token','POST',fields,content_type='application/x-www-form-urlencoded')
    assert replay.status==400,'authorization codes are single-use'
    return token

def claims(token): return json.loads(base64.urlsafe_b64decode(token.split('.')[1]+'==='))
def sha(data): return 'sha256:'+hashlib.sha256(data).hexdigest()
def upload(data,token):
    started=response(REGISTRY+'/v2/project/model/blobs/uploads/','POST',b'',token)
    assert started.status==202,('start upload',started.status,started.read().decode())
    parsed=urllib.parse.urlsplit(urllib.parse.urljoin(REGISTRY,started.headers['Location']))
    url=REGISTRY+parsed.path+('?' + parsed.query if parsed.query else '')+('&' if parsed.query else '?')+urllib.parse.urlencode({'digest':sha(data)})
    saved=response(url,'PUT',data,token,'application/octet-stream')
    assert saved.status==201,('upload',saved.status,saved.read().decode())

def main():
    report=[]
    reader=oauth('reader','repository:project/model:pull repository:project/model:push')
    assert reader['scope']=='repository:project/model:pull',reader['scope']
    report.append('PKCE login with reader downscoped to pull')
    writer=oauth('writer','registry:catalog:read repository:project/model:pull repository:project/model:push','subsoil-browser','http://localhost:5080/explore')
    token=writer['access_token'];identity=claims(token)
    assert identity['iss']==PUBLIC_ISSUER and 'zot' in identity['aud']
    response_manifest=response(REGISTRY+'/v2/project/model/manifests/issued',token=reader['access_token'])
    assert response_manifest.status==200
    raw=response_manifest.read();manifest=json.loads(raw);original=sha(raw)
    layer=manifest['layers'][0]
    pulled=response(REGISTRY+'/v2/project/model/blobs/'+layer['digest'],token=reader['access_token']).read()
    assert sha(pulled)==layer['digest'] and len(pulled)==layer['size']
    report.append('reader pulled digest-verified IFC bytes directly from Zot')
    denied=response(REGISTRY+'/v2/project/model/blobs/uploads/','POST',b'',reader['access_token'])
    assert denied.status in (401,403),denied.status
    assert response(REGISTRY+'/v2/reviews/model/tags/list',token=reader['access_token']).status in (401,403)
    report.append('Zot denied reader uploads and another repository')
    basic=base64.b64encode(b'reader:reader-demo-password').decode()
    endpoint=ISSUER+'/protocol/docker-v2/auth?'+urllib.parse.urlencode({'service':'zot','scope':'repository:project/model:pull,push'})
    with urllib.request.urlopen(urllib.request.Request(endpoint,headers={'Authorization':'Basic '+basic}),timeout=15) as native:
        native=json.load(native)['token']
    assert claims(native)['access'][0]['actions']==['pull']
    assert response(REGISTRY+'/v2/project/model/manifests/issued',token=native).status==200
    report.append('native Distribution clients also use Keycloak with reader-only repository permissions')
    launch_fields=json.dumps({'application_id':'ifc-lite','repository':'project/model','reference':'issued','intent':'edit'}).encode()
    launched=response(SUBSOIL+'/handoff/launches','POST',launch_fields,token,'application/json');assert launched.status==200,(launched.status,launched.read())
    link=json.load(launched)['launch_url'];fragment=urllib.parse.parse_qs(urllib.parse.urlsplit(link).fragment)
    handoff=json.loads(base64.urlsafe_b64decode(fragment['handoff'][0]+'==='))
    assert handoff=={'protocol':'oci-handoff/1','source':{'registry':'http://localhost:5001','repository':'project/model','digest':original},'intent':'edit'}
    report.append('Subsoil resolved the OCI tag and handed off a credential-free immutable source')
    edited=pulled.replace(b'Wall A',b'Edited wall');assert edited!=pulled
    config=b'{}'
    for data in [config,edited]:upload(data,token)
    updated={'schemaVersion':2,'mediaType':'application/vnd.oci.image.manifest.v1+json','artifactType':'application/vnd.ifc-lite.ifc.v1','config':{'mediaType':'application/vnd.oci.empty.v1+json','digest':sha(config),'size':2},'layers':[{'mediaType':'application/x-step','digest':sha(edited),'size':len(edited),'annotations':{'org.opencontainers.image.title':'edited.ifc'}}]}
    raw=json.dumps(updated,separators=(',',':')).encode();tag='smoke-'+secrets.token_hex(4)
    saved=response(REGISTRY+'/v2/project/model/manifests/'+tag,'PUT',raw,token,updated['mediaType']);assert saved.status==201,(saved.status,saved.read())
    assert saved.headers['Docker-Content-Digest']==sha(raw)
    reopened=response(REGISTRY+'/v2/project/model/manifests/'+sha(raw),token=reader['access_token']);assert reopened.read()==raw
    assert response(REGISTRY+'/v2/project/model/blobs/'+sha(edited),token=reader['access_token']).read()==edited
    versions=response(SUBSOIL+'/files/project/model/tags/list',token=token);assert tag in json.load(versions)['tags']
    report.append('writer published an edited IFC to Zot, reopened by digest, and Subsoil displayed the new version')
    unknown=response(ISSUER+'/protocol/openid-connect/token','POST',urllib.parse.urlencode({'grant_type':'client_credentials','client_id':'wrong-audience','client_secret':'wrong-audience-demo-secret'}).encode(),content_type='application/x-www-form-urlencoded')
    unknown=json.load(unknown)['access_token'];assert 'zot' not in claims(unknown).get('aud',[])
    assert response(REGISTRY+'/v2/project/model/manifests/issued',token=unknown).status in (401,403)
    report.append('gateway rejected a genuine Keycloak token with the wrong audience')
    assert response(SUBSOIL+'/v2/project/model/manifests/issued','PUT',b'{}',token,'application/json').status==404
    report.append('Subsoil exposes no OCI storage endpoint')
    print(json.dumps({'passed':report,'version':tag,'digest':sha(raw)},indent=2))
if __name__=='__main__':main()
