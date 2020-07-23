from flask import Blueprint, render_template, request, Response, abort
from srht.api import paginated_response
from srht.config import cfg
from srht.database import db
from srht.flask import csrf_bypass
from srht.validation import Validation
from srht.oauth import oauth, current_token
from buildsrht.runner import queue_build
from buildsrht.types import Artifact, Job, JobStatus, Task, JobGroup
from buildsrht.types import Trigger, TriggerType, TriggerCondition
from buildsrht.manifest import Manifest
import json
import re
import requests
import yaml

api = Blueprint('api', __name__)
csrf_bypass(api)

@api.route("/api/jobs")
@oauth("jobs:read")
def jobs_GET():
    jobs = Job.query.filter(Job.owner_id == current_token.user_id)
    return paginated_response(Job.id, jobs)

@api.route("/api/jobs", methods=["POST"])
@oauth("jobs:write")
def jobs_POST():
    valid = Validation(request)
    _manifest = valid.require("manifest", cls=str)
    max_len = Job.manifest.prop.columns[0].type.length
    valid.expect(not _manifest or len(_manifest) < max_len,
            "Manifest must be less than {} bytes".format(max_len),
            field="manifest")
    note = valid.optional("note", cls=str)
    read = valid.optional("access:read", ["*"], list)
    write = valid.optional("access:write", [current_token.user.username], list)
    secrets = valid.optional("secrets", cls=bool, default=True)
    tags = valid.optional("tags", [], list)
    valid.expect(all(re.match(r"^[A-Za-z0-9_.-]+$", tag) for tag in tags),
        "Invalid tag name, tags must use lowercase alphanumeric characters, underscores, dashes, or dots",
        field="tags")
    triggers = valid.optional("triggers", list(), list)
    execute = valid.optional("execute", True, bool)
    if not valid.ok:
        return valid.response
    try:
        manifest = Manifest(yaml.safe_load(_manifest))
    except Exception as ex:
        valid.error(str(ex))
        return valid.response
    # TODO: access controls
    job = Job(current_token.user, _manifest)
    job.image = manifest.image
    job.note = note
    if tags:
        job.tags = "/".join(tags)
    job.secrets = secrets
    db.session.add(job)
    db.session.flush()
    for task in manifest.tasks:
        t = Task(job, task.name)
        db.session.add(t)
        db.session.flush() # assigns IDs for ordering purposes
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
        queue_build(job, manifest) # commits the session
    else:
        db.session.commit()
    return job.to_dict()

@api.route("/api/jobs/<int:job_id>")
@oauth("jobs:read")
def jobs_by_id_GET(job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    # TODO: ACLs
    return job.to_dict()

@api.route("/api/jobs/<job_id>/artifacts")
@oauth("jobs:read")
def artifacts_by_job_id_GET(job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    artifacts = Artifact.query.filter(Artifact.job_id == job.id)
    return paginated_response(Artifact.id, artifacts)

@api.route("/api/jobs/<job_id>/manifest")
def jobs_by_id_manifest_GET(job_id):
    # TODO: ACLs
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    return Response(job.manifest, content_type="text/plain")

@api.route("/api/jobs/<job_id>/start", methods=["POST"])
@oauth("jobs:write")
def jobs_by_id_start_POST(job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    if job.owner_id != current_token.user_id:
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
    queue_build(job, Manifest(yaml.safe_load(job.manifest)))
    return { }

@api.route("/api/jobs/<int:job_id>/cancel", methods=["POST"])
@oauth("jobs:write")
def jobs_by_id_cancel_POST(job_id):
    job = Job.query.filter(Job.id == job_id).one_or_none()
    if not job:
        abort(404)
    if job.owner_id != current_token.user_id:
        abort(401)
    requests.post(f"http://{job.runner}/job/{job.id}/cancel")
    return { }

@api.route("/api/job-group", methods=["POST"])
@oauth("jobs:write")
def job_group_POST():
    valid = Validation(request)
    jobs = valid.require("jobs")
    valid.expect(not jobs or isinstance(jobs, list) and all(isinstance(j, int) for j in jobs),
            "Expected jobs to be an array of integers (job IDs)")
    triggers = valid.optional("triggers", default=[])
    valid.expect(not triggers or isinstance(triggers, list),
            "Expected triggers to be an array of triggers")
    note = valid.optional("note")

    if not valid.ok:
        return valid.response

    job_group = JobGroup()
    job_group.note = note
    job_group.owner_id = current_token.user_id
    db.session.add(job_group)
    db.session.flush()

    for job_id in jobs:
        job = Job.query.filter(Job.id == job_id).one_or_none()
        valid.expect(job, f"Job ID {job_id} not found")
        if not job:
            continue
        valid.expect(not job.job_group_id,
                f"Job ID {job_id} is already assigned to a job group")
        job.job_group_id = job_group.id
        job_group.jobs.append(job)

    for index, trigger in enumerate(triggers):
        _valid = Validation(trigger)
        action = _valid.require("action", TriggerType)
        condition = _valid.require("condition", TriggerCondition)
        if not _valid.ok:
            _valid.copy(valid, "triggers[{}]".format(index))
            return valid.response
        t = Trigger(job_group)
        t.trigger_type = action
        t.condition = condition
        t.details = json.dumps(trigger)
        db.session.add(t)

    if not valid.ok:
        return valid.response

    db.session.commit()
    return job_group.to_dict()

@api.route("/api/job-group/<int:job_group_id>")
@oauth("jobs:read")
def job_group_by_id_GET(job_group_id):
    job_group = (JobGroup.query
            .filter(JobGroup.id == job_group_id)
            .filter(JobGroup.owner_id == current_token.user_id)).one_or_none()
    if not job_group:
        return {}, 404
    return job_group.to_dict()

@api.route("/api/job-group/<int:job_group_id>/start", methods=["POST"])
@oauth("jobs:read")
def job_group_by_id_start_POST(job_group_id):
    job_group = (JobGroup.query
            .filter(JobGroup.id == job_group_id)
            .filter(JobGroup.owner_id == current_token.user_id)).one_or_none()
    if not job_group:
        return {}, 404
    for job in job_group.jobs:
        if job.status == JobStatus.pending:
            queue_build(job, Manifest(yaml.safe_load(job.manifest)))
    db.session.commit()
    return job_group.to_dict()
