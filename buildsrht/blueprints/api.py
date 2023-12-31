from flask import Blueprint, render_template, request, Response, abort
from srht.api import paginated_response
from srht.config import cfg
from srht.database import db
from srht.flask import csrf_bypass
from srht.graphql import exec_gql
from srht.validation import Validation
from srht.oauth import oauth, current_token
from buildsrht.runner import requires_payment
from buildsrht.types import Artifact, Job, JobStatus, Task, JobGroup
from buildsrht.types import Trigger, TriggerType, TriggerCondition
from buildsrht.manifest import Manifest
import sqlalchemy as sa
import json
import re
import requests
import yaml

api = Blueprint('api', __name__)
csrf_bypass(api)

@api.route("/api/jobs")
@oauth("jobs:read")
def jobs_GET():
    jobs = Job.query.filter(Job.owner_id == current_token.user_id).options(sa.orm.joinedload(Job.tasks))
    return paginated_response(Job.id, jobs)

@api.route("/api/jobs", methods=["POST"])
@oauth("jobs:write")
def jobs_POST():
    valid = Validation(request)
    if requires_payment(current_token.user):
        valid.error("Payment is required to submit build jobs. " +
            "Set up billing at https://meta.sr.ht/billing/initial",
            status=402)
        return valid.response

    _manifest = valid.require("manifest", cls=str)
    max_len = Job.manifest.prop.columns[0].type.length
    valid.expect(not _manifest or len(_manifest) < max_len,
            "Manifest must be less than {} bytes".format(max_len),
            field="manifest")
    note = valid.optional("note", cls=str)
    secrets = valid.optional("secrets", cls=bool)
    tags = valid.optional("tags", [], list)
    valid.expect(not valid.ok or all(re.match(r"^[A-Za-z0-9_.-]+$", tag) for tag in tags),
        "Invalid tag name, tags must use lowercase alphanumeric characters, underscores, dashes, or dots",
        field="tags")
    execute = valid.optional("execute", cls=bool)
    if not valid.ok:
        return valid.response

    resp = exec_gql("builds.sr.ht", """
        mutation SubmitBuild(
            $manifest: String!,
            $note: String,
            $tags: [String!],
            $secrets: Boolean,
            $execute: Boolean,
        ) {
            submit(
                manifest: $manifest,
                note: $note,
                tags: $tags,
                secrets: $secrets,
                execute: $execute,
            ) {
                id
                log {
                    fullURL
                }
                tasks {
                    name
                    status
                    log {
                        fullURL
                    }
                }
                note
                runner
                tags
                owner {
                    canonical_name: canonicalName
                    ... on User {
                        name: username
                    }
                }
            }
        }
    """, user=current_token.user, valid=valid, manifest=_manifest, note=note, tags=tags, secrets=secrets, execute=execute)

    if not valid.ok:
        return valid.response

    resp = resp["submit"]
    if resp["log"]:
        resp["setup_log"] = resp["log"]["fullURL"]
    del resp["log"]
    resp["tags"] = "/".join(resp["tags"])
    for task in resp["tasks"]:
        task["status"] = task["status"].lower()
        if task["log"]:
            task["log"] = task["log"]["fullURL"]
    return resp

@api.route("/api/jobs/<int:job_id>")
@oauth("jobs:read")
def jobs_by_id_GET(job_id):
    job = Job.query.filter(Job.id == job_id).options(sa.orm.joinedload(Job.tasks)).first()
    if not job:
        abort(404)
    # TODO: ACLs
    return job.to_dict()

@api.route("/api/jobs/<int:job_id>/artifacts")
@oauth("jobs:read")
def artifacts_by_job_id_GET(job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    artifacts = Artifact.query.filter(Artifact.job_id == job.id)
    return paginated_response(Artifact.id, artifacts)

@api.route("/api/jobs/<int:job_id>/manifest")
def jobs_by_id_manifest_GET(job_id):
    # TODO: ACLs
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        abort(404)
    return Response(job.manifest, content_type="text/plain")

@api.route("/api/jobs/<int:job_id>/start", methods=["POST"])
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
    exec_gql("builds.sr.ht", """
        mutation StartJob($jobId: Int!) {
            start(jobID: $jobId) { id }
        }
    """, user=current_token.user, jobId=job.id)
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
    execute = valid.optional("execute", default=False)

    if not valid.ok:
        return valid.response

    triggers = [{
        "type": trigger["action"].upper(),
        "condition": trigger["condition"].upper(),
        "email": {
            "to": trigger["to"],
            "cc": trigger.get("cc"),
            "inReplyTo": trigger.get("in_reply_to"),
        } if trigger["action"] == "email" else None,
        "webhook": {
            "url": trigger["url"],
        } if trigger["action"] == "webhook" else None,
    } for trigger in triggers]

    resp = exec_gql("builds.sr.ht", """
        mutation CreateJobGroup($jobIds: [Int!]!, $triggers: [TriggerInput!], $execute: Boolean, $note: String) {
            createGroup(jobIds: $jobIds, triggers: $triggers, execute: $execute, note: $note) {
                id
                note
                owner {
                    canonical_name: canonicalName
                    ... on User {
                        name: username
                    }
                }
                jobs {
                    id
                    status
                }
            }
        }
    """, user=current_token.user, valid=valid, jobIds=jobs, triggers=triggers, execute=execute, note=note)

    if not valid.ok:
        return valid.response

    resp = resp["createGroup"]
    for job in resp["jobs"]:
        job["status"] = job["status"].lower()
    return resp

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
@oauth("jobs:write")
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

@api.route("/api/job-group/<int:job_group_id>/cancel", methods=["POST"])
@oauth("jobs:write")
def job_group_by_id_cancel_POST(job_group_id):
    job_group = (JobGroup.query
            .filter(JobGroup.id == job_group_id)
            .filter(JobGroup.owner_id == current_token.user_id)).one_or_none()
    if not job_group:
        return {}, 404
    for job in job_group.jobs:
        requests.post(f"http://{job.runner}/job/{job.id}/cancel")
    db.session.commit()
    return job_group.to_dict()
