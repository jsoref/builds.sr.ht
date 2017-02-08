from flask import render_template, request
from flask_login import LoginManager, current_user
import locale

from srht.config import cfg, cfgi, load_config
load_config("builds")
from srht.database import DbSession
db = DbSession(cfg("sr.ht", "connection-string"))
#from buildsrht.types import User
db.init()

from srht.flask import SrhtFlask
app = SrhtFlask("builds", __name__)
app.secret_key = cfg("server", "secret-key")
#login_manager = LoginManager()
#login_manager.init_app(app)
#
#@login_manager.user_loader
#def load_user(username):
#    return User.query.filter(User.username == username).first()
#
#login_manager.anonymous_user = lambda: None

try:
    locale.setlocale(locale.LC_ALL, 'en_US')
except:
    pass

@app.route("/")
def index():
    return "hello world"
