from flask import redirect, request, abort
from flask_login import current_user
from functools import wraps
from buildsrht.app import oauth_url
from buildsrht.types import OAuthToken, User
from srht.database import db
from srht.oauth import OAuthScope
from srht.config import cfg
from srht.flask import DATE_FORMAT
from datetime import datetime
import hashlib
import requests
import urllib

meta = cfg("network", "meta")
client_id = cfg("meta.sr.ht", "oauth-client-id")
client_secret = cfg("meta.sr.ht", "oauth-client-secret")
revocation_url = "{}://{}/oauth/revoke".format(
    cfg("server", "protocol"), cfg("server", "domain"))

def loginrequired(f):
    @wraps(f)
    def wrapper(*args, **kwargs):
        if not current_user:
            return redirect(oauth_url(request.url))
        else:
            return f(*args, **kwargs)
    return wrapper

def get_token(token):
    h = hashlib.sha512(token.encode()).hexdigest()
    oauth_token = OAuthToken.query\
            .filter(OAuthToken.token_hash == h).first()
    if oauth_token:
        return oauth_token
    try:
        r = requests.post("{}/oauth/token/{}".format(meta, token), json={
            "client_id": client_id,
            "client_secret": client_secret,
            "revocation_url": revocation_url
        })
        json = r.json()
    except:
        return { "errors": [ { "reason": "Temporary authentication failure" } ] }, 500
    if r.status_code != 200:
        return json
    expires = datetime.strptime(json["expires"], DATE_FORMAT)
    scopes = set(OAuthScope(s) for s in json["scopes"].split(","))
    try:
        r = requests.get("{}/api/user/profile".format(meta, token), headers={
            "Authorization": "token {}".format(token)
        })
        json = r.json()
    except:
        return { "errors": [ { "reason": "Temporary authentication failure" } ] }, 500
    if r.status_code != 200:
        return json
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
    token = OAuthToken(user, token, expires)
    token.scopes = ",".join(str(s) for s in scopes)
    db.session.add(token)
    db.session.commit()
    return token

def oauth(scopes):
    def wrap(f):
        @wraps(f)
        def wrapper(*args, **kwargs):
            token = request.headers.get('Authorization')
            if not token or not token.startswith('token '):
                return { "errors": [ { "reason": "No authorization supplied (expected an OAuth token)" } ] }, 401
            token = token.split(' ')
            if len(token) != 2:
                return { "errors": [ { "reason": "Invalid authorization supplied" } ] }, 401
            token = token[1]
            oauth_token = get_token(token)
            if not isinstance(oauth_token, OAuthToken):
                return oauth_token
            args = (oauth_token,) + args
            required = OAuthScope(scopes)
            available = [OAuthScope(s) for s in oauth_token.scopes.split(',')]
            applicable = [
                s for s in available
                if s.fulfills(required)
            ]
            if not any(applicable):
                return { "errors": [ { "reason": "Your OAuth token is not permitted to use this endpoint (needs {})".format(required) } ] }, 403
            oauth_token.updated = datetime.utcnow()
            db.session.commit()
            return f(*args, **kwargs)
        return wrapper
    return wrap
