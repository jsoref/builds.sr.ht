import json
from celery import Celery
from datetime import datetime
from srht.config import cfg
from srht.database import db
from srht.email import send_email
from srht.oauth import UserType

allow_free = cfg("builds.sr.ht", "allow-free", default="no") == "yes"

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
    sample = json.dumps(manifest.to_dict())
    if "xuirig" in sample or "miner" in sample or "selci" in sample:
        send_email(f"User {job.owner.canonical_name} attempted to submit cryptocurrency mining job #{job.id}",
                cfg("sr.ht", "owner-email"),
                "Cryptocurrency mining attempt on builds.sr.ht")
    else:
        run_build.delay(job.id, manifest.to_dict())

def requires_payment(user):
    if allow_free:
        return False
    # Temporary:
    if user.created < datetime(year=2021, month=5, day=1):
        return False
    return user.user_type not in [
        UserType.admin,
        UserType.active_paying,
        UserType.active_free,
    ]

@runner.task
def run_build(job_id, manifest):
    pass # see worker/context.go
