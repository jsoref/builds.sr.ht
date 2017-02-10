from flask import Blueprint, request, render_template, redirect
from flask_login import login_user, logout_user
from sqlalchemy import or_
from srht.config import cfg
from srht.flask import DATE_FORMAT
from srht.database import db
from buildsrht.types import User
from datetime import datetime
import urllib.parse
import requests

auth = Blueprint('auth', __name__)

meta_uri = cfg("network", "meta")
client_id = cfg("meta.sr.ht", "oauth-client-id")
client_secret = cfg("meta.sr.ht", "oauth-client-secret")

@auth.route("/oauth/callback")
def oauth_callback():
    error = request.args.get("error")
    if error:
        details = request.args.get("details")
        return render_template("oauth-error.html", details=details)
    exchange = request.args.get("exchange")
    scopes = request.args.get("scopes")
    state = request.args.get("state")
    if scopes != "profile:read":
        return render_template("oauth-error.html",
            details="builds.sr.ht requires profile access at a mininum to function correctly. " +
                "To use builds.sr.ht, try again and do not untick these permissions.")
    if not exchange:
        return render_template("oauth-error.html",
            details="Expected an exchange token from meta.sr.ht. Something odd has happened, try again.")
    r = requests.post(meta_uri + "/oauth/exchange", json={
        "client_id": client_id,
        "client_secret": client_secret,
        "exchange": exchange,
    })
    if r.status_code != 200:
        return render_template("oauth-error.html",
            details="Error occured retrieving OAuth token. Try again.")
    json = r.json()
    token = json.get("token")
    expires = json.get("expires")
    if not token or not expires:
        return render_template("oauth-error.html",
            details="Error occured retrieving OAuth token. Try again.")
    expires = datetime.strptime(expires, DATE_FORMAT)

    r = requests.get(meta_uri + "/api/user/profile", headers={
        "Authorization": "token " + token
    })
    if r.status_code != 200:
        return render_template("oauth-error.html",
            details="Error occured retrieving account info. Try again.")
    
    json = r.json()
    user = User.query.filter(or_(User.oauth_token == token,
        User.username == json["username"])).first()
    if not user:
        user = User()
        db.session.add(user)
    user.username = json.get("username")
    user.email = json.get("email")
    user.paid = json.get("paid")
    user.oauth_token = token
    user.oauth_token_expires = expires
    db.session.commit()

    login_user(user)
    if not state or not state.startswith("/"):
        return redirect("/")
    else:
        return redirect(urllib.parse.unquote(state))

@auth.route("/logout")
def logout():
    logout_user()
    return redirect(request.headers.get("Referer") or "/")
