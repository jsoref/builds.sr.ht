from ansi2html import Ansi2HTMLConverter
from flask import Blueprint, render_template, request, abort, redirect
from flask import Response, url_for
from srht.config import cfg
from srht.database import db
from srht.flask import paginate_query, session
from srht.oauth import current_user, loginrequired
from srht.validation import Validation
from buildsrht.types import Job, JobStatus, Task, TaskStatus, User
from buildsrht.manifest import Manifest
from buildsrht.rss import generate_feed
from buildsrht.runner import queue_build
from buildsrht.search import apply_search
from jinja2 import Markup, escape
import hashlib
import requests
import yaml

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
    JobStatus.pending: "text-info",
    JobStatus.queued: "text-info",
    JobStatus.success: "text-success",
    JobStatus.failed: "text-danger",
    JobStatus.running: "text-info icon-spin",
    JobStatus.timeout: "text-danger",
    JobStatus.cancelled: "text-warning",
    TaskStatus.success: "text-success",
    TaskStatus.failed: "text-danger",
    TaskStatus.running: "text-primary icon-spin",
    TaskStatus.pending: "text-info",
    TaskStatus.skipped: "text-muted",
}

icon_map = {
    JobStatus.pending: "clock",
    JobStatus.queued: "clock",
    JobStatus.success: "check",
    JobStatus.failed: "times",
    JobStatus.running: "circle-notch",
    JobStatus.timeout: "clock",
    JobStatus.cancelled: "times",
    TaskStatus.success: "check",
    TaskStatus.failed: "times",
    TaskStatus.running: "circle-notch",
    TaskStatus.pending: "circle",
    TaskStatus.skipped: "minus",
}

def get_jobs(jobs):
    jobs = jobs.order_by(Job.created.desc())
    terms = request.args.get("search")
    jobs = apply_search(jobs, terms)
    return jobs

def jobs_for_feed(jobs):
    jobs = get_jobs(jobs)
    # return only terminated jobs in feed
    terminated_statuses = [
        JobStatus.success,
        JobStatus.cancelled,
        JobStatus.failed,
        JobStatus.timeout,
    ]
    return jobs.filter(Job.status.in_(terminated_statuses))

def jobs_page(jobs, sidebar="sidebar.html", **kwargs):
    jobs, pagination = paginate_query(get_jobs(jobs))
    search = request.args.get("search")
    return render_template("jobs.html",
        jobs=jobs, status_map=status_map, icon_map=icon_map, tags=tags,
        sort_tasks=lambda tasks: sorted(tasks, key=lambda t: t.id),
        sidebar=sidebar, search=search, **pagination, **kwargs
    )

def jobs_feed(jobs, title, endpoint, **urlvalues):
    jobs = jobs_for_feed(jobs)
    description = title
    origin = cfg("builds.sr.ht", "origin")
    link = origin + url_for(endpoint, **urlvalues)
    return generate_feed(jobs, title, link, description)

badge_success = """
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="124" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="124" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h71v20H0z"/><path fill="#4c1" d="M71 0h53v20H71z"/><path fill="url(#b)" d="M0 0h124v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"> <text x="365" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="610">__NAME__</text><text x="365" y="140" transform="scale(.1)" textLength="610">__NAME__</text><text x="965" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="430">success</text><text x="965" y="140" transform="scale(.1)" textLength="430">success</text></g></svg>
"""

badge_failure = """
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="124" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="124" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h71v20H0z"/><path fill="#e05d44" d="M71 0h53v20H71z"/><path fill="url(#b)" d="M0 0h124v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"> <text x="365" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="610">__NAME__</text><text x="365" y="140" transform="scale(.1)" textLength="610">__NAME__</text><text x="965" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="430">failure</text><text x="965" y="140" transform="scale(.1)" textLength="430">failure</text></g></svg>
"""

badge_unknown = """
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="132" height="20"><linearGradient id="b" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="a"><rect width="132" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#a)"><path fill="#555" d="M0 0h71v20H0z"/><path fill="#9f9f9f" d="M71 0h61v20H71z"/><path fill="url(#b)" d="M0 0h132v20H0z"/></g><g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="110"> <text x="365" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="610">__NAME__</text><text x="365" y="140" transform="scale(.1)" textLength="610">__NAME__</text><text x="1005" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="510">unknown</text><text x="1005" y="140" transform="scale(.1)" textLength="510">unknown</text></g> </svg>
"""

def svg_page(jobs):
    name = request.args.get("name",
            default=cfg("sr.ht", "site-name"))
    job = (get_jobs(jobs)
        .filter(Job.status.in_([
            JobStatus.success,
            JobStatus.failed,
            JobStatus.timeout]))
        .first())
    if not job:
        badge = badge_unknown.replace("__NAME__", name)
    elif job.status == JobStatus.success:
        badge = badge_success.replace("__NAME__", name)
    else:
        badge = badge_failure.replace("__NAME__", name)
    return Response(badge, mimetype="image/svg+xml", headers={
        "Cache-Control": "no-cache",
        "ETag": hashlib.sha1(badge.encode()).hexdigest(),
    })

@jobs.route("/")
def index():
    if not current_user:
        return render_template("index-logged-out.html")
    return jobs_page(
            Job.query.filter(Job.owner_id == current_user.id),
            "index.html")

@jobs.route("/submit")
@loginrequired
def submit_GET():
    manifest = session.get("manifest")
    if manifest:
        del session["manifest"]
    return render_template("submit.html", manifest=manifest)

@jobs.route("/resubmit/<int:job_id>")
@loginrequired
def resubmit_GET(job_id):
    job = Job.query.filter(Job.id == job_id).one_or_none()
    if not job:
        abort(404)
    session["manifest"] = job.manifest
    return redirect("/submit")

@jobs.route("/submit", methods=["POST"])
@loginrequired
def submit_POST():
    valid = Validation(request)
    _manifest = valid.require("manifest", friendly_name="Manifest")
    max_len = Job.manifest.prop.columns[0].type.length
    note = valid.optional("note", default="Submitted on the web")
    valid.expect(not _manifest or len(_manifest) < max_len,
            "Manifest must be less than {} bytes".format(max_len),
            field="manifest")
    if not valid.ok:
        return render_template("submit.html", **valid.kwargs)
    try:
        manifest = Manifest(yaml.safe_load(_manifest))
    except Exception as ex:
        valid.error(str(ex), field="manifest")
        return render_template("submit.html", **valid.kwargs)
    job = Job(current_user, _manifest)
    job.image = manifest.image
    job.note = note
    db.session.add(job)
    db.session.flush()
    for task in manifest.tasks:
        t = Task(job, task.name)
        db.session.add(t)
        db.session.flush() # assigns IDs for ordering purposes
    queue_build(job, manifest) # commits the session
    return redirect("/~" + current_user.username + "/job/" + str(job.id))

@jobs.route("/cancel/<int:job_id>", methods=["POST"])
@loginrequired
def cancel(job_id):
    job = Job.query.filter(Job.id == job_id).one_or_none()
    if not job:
        abort(404)
    if job.owner_id != current_user.id:
        abort(401)
    requests.post(f"http://{job.runner}/job/{job.id}/cancel")
    return redirect("/~" + current_user.username + "/job/" + str(job.id))

@jobs.route("/~<username>")
def user(username):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)
    if not current_user or current_user.id != user.id:
        pass # TODO: access controls
    return jobs_page(jobs, user=user, breadcrumbs=[
        { "name": "~" + user.username, "link": "" }
    ])

@jobs.route("/~<username>/rss.xml")
def user_rss(username):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)
    if not current_user or current_user.id != user.id:
        pass  # TODO: access controls
    return jobs_feed(jobs, f"{user.username}'s jobs",
                     "jobs.user", username=username)

@jobs.route("/~<username>.svg")
def user_svg(username):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)
    return svg_page(jobs)

@jobs.route("/~<username>/<path:path>")
def tag(username, path):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)\
        .filter(Job.tags.ilike(path + "%"))
    if not current_user or current_user.id != user.id:
        pass # TODO: access controls
    return jobs_page(jobs, user=user, breadcrumbs=[
        { "name": "~" + user.username, "url": "" }
    ] + tags(path))

@jobs.route("/~<username>/<path:path>/rss.xml")
def tag_rss(username, path):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)\
        .filter(Job.tags.ilike(path + "%"))
    if not current_user or current_user.id != user.id:
        pass  # TODO: access controls
    base_title = "/".join([f"~{user.username}"] +
                          [t["name"] for t in tags(path)])
    return jobs_feed(jobs, base_title + " jobs",
                     "jobs.tag", username=username, path=path)

@jobs.route("/~<username>/<path:path>.svg")
def tag_svg(username, path):
    user = User.query.filter(User.username == username).first()
    if not user:
        abort(404)
    jobs = Job.query.filter(Job.owner_id == user.id)\
        .filter(Job.tags.ilike(path + "%"))
    return svg_page(jobs)

log_max = 131072

ansi = Ansi2HTMLConverter(scheme="mint-terminal", linkify=True)

def logify(text, task, log_url):
    text = ansi.convert(text, full=False)
    if len(text) >= log_max:
        text = text[-log_max:]
        try:
            text = text[text.index('\n')+1:]
        except ValueError:
            pass
        nlines = len(text.splitlines())
        text = (Markup('<pre>')
                + Markup('<span class="text-muted">'
                    'This is a big file! Only the last 128KiB is shown. '
                    f'<a target="_blank" href="{escape(log_url)}">'
                        'Click here to download the full log</a>.'
                    '</span>\n\n')
                + Markup(text)
                + Markup('</pre>'))
        linenos = Markup('<pre>\n\n\n')
    else:
        nlines = len(text.splitlines())
        text = (Markup('<pre>')
                + Markup(text)
                + Markup('</pre>'))
        linenos = Markup('<pre>')
    for no in range(1, nlines + 1):
        linenos += Markup(f'<a href="#{escape(task)}-{no-1}" '
                + f'id="#{escape(task)}-{no-1}">{no}</a>')
        if no != nlines:
            linenos += Markup("\n")
    linenos += Markup("</pre>")
    return (Markup('<td>')
            + linenos
            + Markup('</td><td>')
            + Markup(ansi.produce_headers())
            + text
            + Markup('</td>'))

@jobs.route("/~<username>/job/<int:job_id>")
def job_by_id(username, job_id):
    # TODO: maybe we want per-user job IDs
    job = Job.query.get(job_id)
    if not job:
        abort(404)
    logs = list()
    build_user = cfg("git.sr.ht::dispatch", "/usr/bin/buildsrht-keys", "builds:builds").split(":")[0]
    def get_log(log_url, name):
        try:
            r = requests.head(log_url)
            cl = int(r.headers["Content-Length"])
            if cl > log_max:
                r = requests.get(log_url, headers={
                    "Range": f"bytes={cl-log_max}-{cl-1}",
                }, timeout=3)
            else:
                r = requests.get(log_url, timeout=3)
            if r.status_code >= 200 and r.status_code <= 299:
                logs.append({
                    "name": name,
                    "log": logify(r.content.decode('utf-8'),
                        "task-" + name if name else "setup", log_url)
                })
                return True
            else:
                raise Exception()
        except:
            logs.append({
                "name": name,
                "log": Markup('<td></td><td><pre><strong class="text-danger">'
                    f'Error fetching logs for task "{escape(name)}"</strong>'
                    '</pre></td>')
            })
            return False
    log_url = "http://{}/logs/{}/log".format(job.runner, job.id)
    if get_log(log_url, None):
        for task in sorted(job.tasks, key=lambda t: t.id):
            if task.status == TaskStatus.pending:
                continue
            log_url = "http://{}/logs/{}/{}/log".format(
                    job.runner, job.id, task.name)
            if not get_log(log_url, task.name):
                break
    return render_template("job.html",
            job=job,
            build_user=build_user,
            status_map=status_map,
            icon_map=icon_map,
            logs=logs,
            sort_tasks=lambda tasks: sorted(tasks, key=lambda t: t.id))
