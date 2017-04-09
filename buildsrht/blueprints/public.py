from flask import Blueprint, render_template, request
from flask_login import current_user
from srht.database import db
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build

public = Blueprint('public', __name__)

@public.route("/")
def index():
    return render_template("index.html")
