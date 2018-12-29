from srht.config import cfg
from srht.oauth import OAuthScope, AbstractOAuthService, DelegatedScope
from srht.flask import DATE_FORMAT
from srht.database import db
from buildsrht.types import OAuthToken, User, UserType
from datetime import datetime

client_id = cfg("builds.sr.ht", "oauth-client-id")
client_secret = cfg("builds.sr.ht", "oauth-client-secret")
revocation_url = "{}/oauth/revoke".format(cfg("builds.sr.ht", "origin"))

class BuildOAuthService(AbstractOAuthService):
    def __init__(self):
        super().__init__(client_id, client_secret, delegated_scopes=[
            DelegatedScope("jobs", "build jobs", True),
        ])

    def get_token(self, token, token_hash, scopes):
        now = datetime.utcnow()
        oauth_token = (OAuthToken.query
                .filter(OAuthToken.token_hash == token_hash)
                .filter(OAuthToken.expires > now)
        ).first()
        if oauth_token:
            return oauth_token
        _token, profile = self.exchange(token, revocation_url)
        expires = datetime.strptime(_token["expires"], DATE_FORMAT)
        scopes = set(OAuthScope(s) for s in _token["scopes"].split(","))
        user = User.query.filter(User.username == profile["name"]).one_or_none()
        if not user:
            user = User()
            user.username = profile["name"]
            user.email = profile["email"]
            user.user_type = UserType(profile["user_type"])
            user.oauth_token = token
            user.oauth_token_expires = expires
            db.session.add(user)
            db.session.flush()
        oauth_token = OAuthToken(user, token, expires)
        oauth_token.scopes = ",".join(str(s) for s in scopes)
        db.session.add(oauth_token)
        db.session.commit()
        return oauth_token

    def lookup_or_register(self, exchange, profile, scopes):
        user = User.query.filter(User.username == profile["name"]).one_or_none()
        if not user:
            user = User()
            db.session.add(user)
        user.username = profile["name"]
        user.email = profile["email"]
        user.user_type = UserType(profile["user_type"])
        user.oauth_token = exchange["token"]
        user.oauth_token_expires = exchange["expires"]
        user.oauth_token_scopes = scopes
        db.session.commit()
        return user
