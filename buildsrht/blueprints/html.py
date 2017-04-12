from flask import Blueprint, render_template, request
from flask_login import current_user
from srht.database import db
from buildsrht.types import Job
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build

html = Blueprint('html', __name__)

@html.route("/")
def index():
    if not current_user:
        jobs = list()
    else:
        jobs = Job.query\
            .filter(Job.owner_id == current_user.id)\
            .order_by(Job.updated.desc())\
            .limit(10).all()
    return render_template("index.html", jobs=jobs)
