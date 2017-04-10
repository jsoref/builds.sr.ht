from flask import Blueprint, render_template, request, Response
from flask_login import current_user
from srht.database import db
from srht.validation import Validation
from buildsrht.decorators import oauth
from buildsrht.runner import queue_build
from buildsrht.types import Job, JobStatus, Task
from buildsrht.types import Trigger, TriggerType, TriggerCondition
from buildsrht.manifest import Manifest
import json

api = Blueprint('api', __name__)

@api.route("/api/jobs", methods=["POST"])
@oauth("jobs:write")
def jobs_POST(token):
    valid = Validation(request)
    _manifest = valid.require("manifest", str)
    note = valid.optional("note", str)
    read = valid.optional("access:read", list, default=["*"])
    write = valid.optional("access:write", list, default=[token.user.username])
    triggers = valid.optional("triggers", list, default=list())
    execute = valid.optional("execute", bool, default=True)
    if not valid.ok:
        return valid.response
    try:
        manifest = Manifest(_manifest)
    except ex:
        valid.error(ex.message)
        return valid.response
    # TODO: access controls
    job = Job(token.user, _manifest)
    job.note = note
    db.session.add(job)
    db.session.flush()
    for task in manifest.tasks:
        t = Task(job, task.name)
        db.session.add(t)
    for index, trigger in enumerate(triggers):
        _valid = Validation(trigger)
        action = _valid.require("action", TriggerType)
        condition = _valid.require("condition", TriggerCondition)
        if not _valid.ok:
            _valid.copy(valid, "triggers[{}]".format(index))
            return valid.response
        # TODO: Validate details based on trigger type
        t = Trigger(job)
        t.trigger_type = action
        t.condition = condition
        t.details = json.dumps(trigger)
        db.session.add(t)
    if execute:
        queue_build(job) # commits the session
    else:
        db.session.commit()
    return {
        "id": job.id
    }

@api.route("/api/jobs/<job_id>")
@oauth("jobs:read")
def jobs_by_id_GET(token, job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    # TODO: ACLs
    return {
        "id": job.id,
        "status": job.status.value,
        "setup_log": "http://{}/logs/{}/log".format(job.runner, job.id),
        "tasks": [
            {
                "name": task.name,
                "status": task.status.value,
                "log": "http://{}/logs/{}/{}/log".format(
                    job.runner, job.id, task.name)
            } for task in job.tasks
        ]
    }

@api.route("/api/jobs/<job_id>/manifest")
@oauth("jobs:read")
def jobs_by_id_manifest_GET(token, job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    return Response(job.manifest, content_type="text/plain")

@api.route("/api/jobs/<job_id>/start", methods=["POST"])
@oauth("jobs:write")
def jobs_by_id_start_POST(token, job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    if job.owner_id != token.user_id:
        abort(401) # TODO: ACLs
    if job.status != JobStatus.pending:
        reason_map = {
            JobStatus.queued: "queued",
            JobStatus.running: "running",
            JobStatus.success: "complete",
            JobStatus.failed: "complete",
        }
        return {
            "errors": [
                { "reason": "This job is already {}".format(reason_map.get(job.status)) }
            ]
        }, 400
    queue_build(job)
    return { }
