from flask import Blueprint, render_template, request
from flask_login import current_user
from srht.database import db
from buildsrht.decorators import oauth
from buildsrht.runner import run_build

api = Blueprint('api', __name__)

@api.route("/api/jobs")#, methods=["POST"])
@oauth("jobs:read")
def jobs_POST():
    return { "it": "worked" }
