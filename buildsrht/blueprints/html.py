from flask import Blueprint, render_template, request, abort
from flask_login import current_user
from srht.database import db
from buildsrht.types import Job, JobStatus, TaskStatus, User
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build

html = Blueprint("html", __name__)

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
        status_map={
            JobStatus.success: "text-success",
            JobStatus.failed: "text-danger",
            JobStatus.running: "text-black",
            TaskStatus.success: "text-success",
            TaskStatus.failed: "text-danger",
            TaskStatus.running: "text-black",
            TaskStatus.pending: "text-muted",
        },
        sort_tasks=lambda tasks: sorted(tasks, key=lambda t: t.id),
        total_pages=total_pages,
        page=page+1,
        tags=tags,
        sidebar=sidebar,
        **kwargs
    )

@html.route("/")
def index():
    if not current_user:
        return render_template("index-logged-out.html")
    return jobs_page(Job.query.filter(Job.owner_id == current_user.id), "index.html")

@html.route("/jobs/~<username>")
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

@html.route("/jobs/~<username>/<path:path>")
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
