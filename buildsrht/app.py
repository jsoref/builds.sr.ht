from flask import session
from srht.flask import SrhtFlask
from srht.config import cfg
from srht.database import DbSession

db = DbSession(cfg("builds.sr.ht", "connection-string"))

from buildsrht.types import User, JobStatus, UserType

db.init()

from buildsrht.oauth import BuildOAuthService

class BuildApp(SrhtFlask):
    def __init__(self):
        super().__init__("builds.sr.ht", __name__,
                oauth_service=BuildOAuthService())

        from buildsrht.blueprints.api import api
        from buildsrht.blueprints.jobs import jobs
        from buildsrht.blueprints.secrets import secrets

        self.register_blueprint(api)
        self.register_blueprint(jobs)
        self.register_blueprint(secrets)

        @self.context_processor
        def inject():
            return { "JobStatus": JobStatus }

        @self.login_manager.user_loader
        def load_user(username):
            # TODO: Switch to a session token
            return User.query.filter(User.username == username).one_or_none()

app = BuildApp()
