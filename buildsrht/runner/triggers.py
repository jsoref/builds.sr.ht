from buildsrht.manifest import TriggerAction, TriggerCondition
from buildsrht.types import JobStatus
import requests

def process_triggers(manifest, job):
    print("Executing triggers")
    for trigger in manifest.triggers:
        if (trigger.condition == TriggerCondition.success and
                job.status == [JobStatus.failed]):
            continue
        if (trigger.condition == TriggerCondition.failure and
                job.status == [JobStatus.success]):
            continue
        if trigger.action == TriggerAction.webhook:
            url = trigger.attrs.get("url")
            if url:
                print("Webhook: ", url)
                requests.post(url, json=job.to_dict())
