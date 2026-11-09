"""Fetch Keycloak's public certificate and configure Zot's native Distribution verifier."""
import base64, hashlib, json, pathlib, time, urllib.request
for attempt in range(120):
    try:
        with urllib.request.urlopen('http://keycloak:8080/realms/subsoil/protocol/openid-connect/certs', timeout=5) as response:
            keys = json.load(response)['keys']
        key = next(key for key in keys if key.get('alg') == 'RS256' and key.get('use') == 'sig')
        # Keycloak's Docker protocol uses the Distribution/libtrust key identifier.
        # Publish that alias alongside OIDC kids for the same public signing keys.
        def der(tag, data):
            length=len(data)
            encoded=bytes([length]) if length<128 else bytes([128+(length.bit_length()+7)//8])+length.to_bytes((length.bit_length()+7)//8,'big')
            return bytes([tag])+encoded+data
        def integer(encoded):
            value=base64.urlsafe_b64decode(encoded+'===').lstrip(b'\0')
            return der(2,(b'\0' if value[0]&128 else b'')+value)
        jwks=list(keys)
        for rsa in keys:
            if rsa.get('kty')!='RSA': continue
            public=der(48,bytes.fromhex('300d06092a864886f70d0101010500')+der(3,b'\0'+der(48,integer(rsa['n'])+integer(rsa['e']))))
            kid=base64.b32encode(hashlib.sha256(public).digest()[:30]).decode().rstrip('=')
            alias=dict(rsa);alias['kid']=':'.join(kid[i:i+4] for i in range(0,len(kid),4));jwks.append(alias)
        pathlib.Path('/config/jwks.json').write_text(json.dumps({'keys':jwks}))
        cert = key['x5c'][0]
        pathlib.Path('/config/public.pem').write_text('-----BEGIN CERTIFICATE-----\n'+'\n'.join(cert[i:i+64] for i in range(0,len(cert),64))+'\n-----END CERTIFICATE-----\n')
        config={'distSpecVersion':'1.1.1','storage':{'rootDirectory':'/var/lib/zot'},'http':{'address':'0.0.0.0','port':'5000','auth':{'bearer':{'realm':'http://localhost:8080/realms/subsoil/protocol/docker-v2/auth','service':'zot','cert':'/config/public.pem'}}},'log':{'level':'warn'}}
        pathlib.Path('/config/zot.json').write_text(json.dumps(config))
        print('Configured Zot with the Keycloak realm public certificate.', flush=True)
        break
    except Exception as error:
        if attempt == 119: raise
        time.sleep(1)
