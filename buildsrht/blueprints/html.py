from flask import Blueprint, render_template, request
from flask_login import current_user
from srht.database import db
from buildsrht.types import Job, JobStatus, TaskStatus
from buildsrht.decorators import loginrequired
from buildsrht.runner import run_build

html = Blueprint("html", __name__)

def tags(job):
    if not job.tags:
        return list()
    previous = list()
    results = list()
    for tag in job.tags.split("/"):
        results.append({
            "name": tag,
            "url": "/".join(previous + [tag])
        })
        previous.append(tag)
    print(results)
    return results

@html.route("/")
def index():
    if not current_user:
        return render_template("index-logged-out.html")
    page = request.args.get("page")
    jobs = Job.query\
        .filter(Job.owner_id == current_user.id)\
        .order_by(Job.updated.desc())
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
    return render_template("index.html",
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
        tags=tags
    )
