from flask import Blueprint, render_template, request
from flask_login import current_user
from srht.database import db
from buildsrht.types import Build
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build

public = Blueprint('public', __name__)

@public.route("/")
def index():
    return render_template("index.html")

@public.route("/adhoc", methods=['POST'])
@loginrequired
def adhoc():
    build = Build(request.form.get("manifest"))
    build.owner_id = current_user.id
    build.source = "Manually created by " + current_user.username
    db.session.add(build)
    db.session.commit()
    run_build.delay(build.id)
    return str(build.id)
