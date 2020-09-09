import json
from celery import Celery
from srht.config import cfg
from srht.database import db
from srht.email import send_email

runner = Celery('builds', broker=cfg("builds.sr.ht", "redis"), config_source={
    "CELERY_TASK_SERIALIZER": "json",
    "CELERY_ACCEPT_CONTENT": ["json"],
    "CELERY_RESULT_SERIALIZER": "json",
    "CELERY_ENABLE_UTC": True,
    "CELERY_TASK_PROTOCOL": 1
})

def queue_build(job, manifest):
    from buildsrht.types import JobStatus
    job.status = JobStatus.queued
    db.session.commit()
    # crypto mining attempt
    # pretend to accept it and let the admins know
    if "xuirig" in json.dumps(manifest.to_dict()):
        send_email(f"User {job.owner.canonical_name} attempted to submit cryptocurrency mining job #{job.id}",
                cfg("sr.ht", "owner-email"),
                "Cryptocurrency mining attempt on builds.sr.ht")
    else:
        run_build.delay(job.id, manifest.to_dict())

@runner.task
def run_build(job_id, manifest):
    pass # see worker/context.go
