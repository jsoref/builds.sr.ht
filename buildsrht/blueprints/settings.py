from flask import Blueprint, current_app, render_template, request, url_for, abort, redirect
from flask import current_app
from srht.database import db
from srht.oauth import current_user, loginrequired
from srht.validation import Validation
from buildsrht.types import Job, Visibility

settings = Blueprint("settings", __name__)

@settings.route("/~<username>/job/<int:job_id>/settings/details")
@loginrequired
def details_GET(username, job_id):
    job = Job.query.get(job_id)
    if not job:
        abort(404)
    if current_user.id != job.owner_id:
        abort(404)
    return render_template("job-details.html",
        view="details", job=job)

@settings.route("/~<username>/job/<int:job_id>/settings/details", methods=["POST"])
@loginrequired
def details_POST(username, job_id):
    job = Job.query.get(job_id)
    if not job:
        abort(404)
    if current_user.id != job.owner_id:
        abort(404)

    valid = Validation(request)
    visibility = valid.require("visibility")
    if not valid.ok:
        return render_template("job-details.html",
            job=job, **valid.kwargs), 400

    # TODO: GraphQL mutation to update job details
    job.visibility = visibility
    db.session.commit()

    return redirect(url_for("settings.details_GET",
        username=job.owner.username,
        job_id=job.id))
