import json
from celery import Celery
from datetime import datetime
from srht.config import cfg
from srht.database import db
from srht.email import send_email
from srht.graphql import exec_gql
from srht.oauth import UserType
from srht.metrics import RedisQueueCollector
from prometheus_client import Counter

allow_free = cfg("builds.sr.ht", "allow-free", default="no") == "yes"

builds_broker = cfg("builds.sr.ht", "redis")
runner = Celery('builds', broker=builds_broker, config_source={
    "CELERY_TASK_SERIALIZER": "json",
    "CELERY_ACCEPT_CONTENT": ["json"],
    "CELERY_RESULT_SERIALIZER": "json",
    "CELERY_ENABLE_UTC": True,
    "CELERY_TASK_PROTOCOL": 1
})

builds_queue_metrics_collector = RedisQueueCollector(builds_broker, "buildsrht_builds", "Number of builds currently in queue")
builds_submitted = Counter("buildsrht_builds_submited", "Number of builds submitted")

def submit_build(user, manifest, note=None, tags=[]):
    resp = exec_gql("builds.sr.ht", """
        mutation SubmitBuild($manifest: String!, $tags: [String!], $note: String) {
            submit(manifest: $manifest, tags: $tags, note: $note) {
                id
            }
        }
    """, user=user, manifest=manifest, note=note, tags=tags)
    return resp["submit"]["id"]

def requires_payment(user):
    if allow_free:
        return False
    return user.user_type not in [
        UserType.admin,
        UserType.active_paying,
        UserType.active_free,
    ]

@runner.task
def run_build(job_id, manifest):
    pass # see worker/context.go
