from srht.config import cfg
from srht.oauth import OAuthScope, AbstractOAuthService, set_base_service
from srht.flask import DATE_FORMAT
from srht.database import db
from buildsrht.types import OAuthToken, User
from datetime import datetime
from json import dumps
import requests

meta = cfg("network", "meta")
client_id = cfg("meta.sr.ht", "oauth-client-id")
client_secret = cfg("meta.sr.ht", "oauth-client-secret")
revocation_url = "{}://{}/oauth/revoke".format(
    cfg("server", "protocol"), cfg("server", "domain"))

class BuildOAuthService(AbstractOAuthService):
    def get_client_id(self):
        return client_id

    def get_token(self, token, token_hash, scopes):
        now = datetime.utcnow()
        oauth_token = (OAuthToken.query
                .filter(OAuthToken.token_hash == token_hash)
                .filter(OAuthToken.expires > now)
        ).first()
        if oauth_token:
            return oauth_token
        try:
            r = requests.post("{}/oauth/token/{}".format(meta, token), json={
                "client_id": client_id,
                "client_secret": client_secret,
                "revocation_url": revocation_url
            })
            json = r.json()
        except Exception as ex:
            print(ex)
            # TODO: Return 500
            raise Exception("Temporary authentication failure")
        if r.status_code != 200:
            # TODO: ResponseException to include an explicit response?
            raise Exception("meta.sr.ht error: " + dumps(json))
        expires = datetime.strptime(json["expires"], DATE_FORMAT)
        scopes = set(OAuthScope(s) for s in json["scopes"].split(","))
        try:
            r = requests.get("{}/api/user/profile".format(meta), headers={
                "Authorization": "token {}".format(token)
            })
            json = r.json()
        except Exception as ex:
            # TODO: Return 500
            print(ex)
            raise Exception("Temporary authentication failure")
        if r.status_code != 200:
            # TODO
            raise Exception("meta.sr.ht error: " + dumps(json))
        user = User.query.filter(User.username == json["username"]).first()
        if not user:
            user = User()
            user.username = json.get("username")
            user.email = json.get("email")
            user.paid = json.get("paid")
            user.oauth_token = token
            user.oauth_token_expires = expires
            db.session.add(user)
            db.session.flush()
        oauth_token = OAuthToken(user, token, expires)
        oauth_token.scopes = ",".join(str(s) for s in scopes)
        db.session.add(oauth_token)
        db.session.commit()
        return oauth_token

set_base_service(BuildOAuthService())
