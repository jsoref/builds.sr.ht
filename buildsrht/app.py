from flask import session
from srht.flask import SrhtFlask
from srht.config import cfg
from srht.database import DbSession

db = DbSession(cfg("builds.sr.ht", "connection-string"))

from buildsrht.types import User, JobStatus

db.init()

import buildsrht.oauth

class BuildApp(SrhtFlask):
    def __init__(self):
        super().__init__("builds.sr.ht", __name__)

        from buildsrht.blueprints.api import api
        from buildsrht.blueprints.jobs import jobs
        from buildsrht.blueprints.secrets import secrets

        self.register_blueprint(api)
        self.register_blueprint(jobs)
        self.register_blueprint(secrets)

        meta_client_id = cfg("builds.sr.ht", "oauth-client-id")
        meta_client_secret = cfg("builds.sr.ht", "oauth-client-secret")
        self.configure_meta_auth(meta_client_id, meta_client_secret)

        @self.context_processor
        def inject():
            return { "JobStatus": JobStatus }

        @self.login_manager.user_loader
        def load_user(username):
            # TODO: Switch to a session token
            return User.query.filter(User.username == username).first()

    def lookup_or_register(self, exchange, profile, scopes):
        user = User.query.filter(User.username == profile["username"]).first()
        if not user:
            user = User()
            db.session.add(user)
        user.username = profile.get("username")
        user.email = profile.get("email")
        user.paid = profile.get("paid")
        user.oauth_token = exchange["token"]
        user.oauth_token_expires = exchange["expires"]
        user.oauth_token_scopes = scopes
        db.session.commit()
        return user

app = BuildApp()
