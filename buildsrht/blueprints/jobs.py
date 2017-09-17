from flask import Blueprint, render_template, request, abort
from flask_login import current_user
from srht.database import db
from buildsrht.types import Job, JobStatus, TaskStatus, User
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build
import requests

jobs = Blueprint("jobs", __name__)

def tags(tags):
    if not tags:
        return list()
    previous = list()
    results = list()
    for tag in tags.split("/"):
        results.append({
            "name": tag,
            "url": "/" + "/".join(previous + [tag])
        })
        previous.append(tag)
    return results

status_map = {
    JobStatus.success: "status-text text-success",
    JobStatus.failed: "status-text text-danger",
    JobStatus.running: "status-text text-info",
    TaskStatus.success: "status-text text-success",
    TaskStatus.failed: "status-text text-danger",
    TaskStatus.running: "status-text text-primary",
    TaskStatus.pending: "status-text text-black",
    TaskStatus.skipped: "status-text text-muted",
}

def jobs_page(jobs, sidebar, **kwargs):
    jobs = jobs.order_by(Job.updated.desc())
    page = request.args.get("page")
    total_jobs = jobs.count()
    total_pages = jobs.count() // 10 + 1
    if total_jobs % 10 == 0:
        total_pages -= 1
    if page is not None:
        try:
            page = int(page) - 1
            jobs = jobs.offset(page * 10)
        except:
            page = 0
    else:
        page = 0
    jobs = jobs.limit(10).all()
    return render_template("jobs.html",
        jobs=jobs,
        status_map=status_map,
        sort_tasks=lambda tasks: sorted(tasks, key=lambda t: t.id),
        total_pages=total_pages,
        page=page+1,
        tags=tags,
        sidebar=sidebar,
        **kwargs
    )

@jobs.route("/")
def index():
    if not current_user:
        return render_template("index-logged-out.html")
    return jobs_page(Job.query.filter(Job.owner_id == current_user.id), "index.html")

@jobs.route("/jobs/~<username>")
def user(username):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)
    if not current_user or current_user.id != user.id:
        pass # TODO: access controls
    return jobs_page(jobs, "user.html", user=user, breadcrumbs=[
        { "name": "~" + user.username, "link": "" }
    ])

@jobs.route("/jobs/~<username>/<path:path>")
def tag(username, path):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)\
        .filter(Job.tags.ilike(path + "%"))
    if not current_user or current_user.id != user.id:
        pass # TODO: access controls
    return jobs_page(jobs, "user.html", user=user, breadcrumbs=[
        { "name": "~" + user.username, "url": "" }
    ] + tags(path))

@jobs.route("/job/<int:job_id>")
def job_by_id(job_id):
    job = Job.query.get(job_id)
    if not job:
        abort(404)
    logs = list()
    # TODO: cache this shit
    r = requests.get("http://{}/logs/{}/log".format(job.runner, job.id))
    if r.status_code == 200:
        logs.append({
            "name": None,
            "log": r.text.splitlines()
        })
    for task in sorted(job.tasks, key=lambda t: t.id):
        if task.status == TaskStatus.pending:
            continue
        r = requests.get("http://{}/logs/{}/{}/log".format(job.runner,
            job.id, task.name))
        if r.status_code == 200:
            logs.append({
                "name": task.name,
                "log": r.text.splitlines()
            })
    return render_template("job.html",
            job=job,
            status_map=status_map,
            logs=logs,
            sort_tasks=lambda tasks: sorted(tasks, key=lambda t: t.id))
