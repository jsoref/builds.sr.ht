from celery import Celery
from redis import Redis
from srht.config import cfg
from srht.database import DbSession, db
from buildsrht.manifest import Manifest
import os
import shlex
import subprocess

redis = None

runner = Celery('builds', broker=cfg("builds.sr.ht", "redis"))

def queue_build(job, manifest):
    from buildsrht.types import JobStatus
    job.status = JobStatus.queued
    db.session.commit()
    run_build.delay(job.id, manifest.to_dict())

@runner.task
def run_build(job_id, manifest):
    global redis
    redis = Redis()

    db = DbSession(cfg("builds.sr.ht", "connection-string"))
    from buildsrht.types import Job, JobStatus, TaskStatus
    db.init()
    from buildsrht.runner.context import BuildContext
    from buildsrht.runner.tasks import early_setup_tasks, setup_tasks
    from buildsrht.runner.triggers import process_triggers

    job = Job.query.filter(Job.id == job_id).one_or_none()
    job.runner = cfg("builds.sr.ht::worker", "name")
    job.status = JobStatus.running
    db.session.commit()

    manifest = Manifest(manifest)
    context = BuildContext(job, manifest)

    buildlogs = cfg("builds.sr.ht::worker", "buildlogs")
    logs = os.path.join(buildlogs, str(job.id))
    os.makedirs(logs)
    for task in manifest.tasks:
        os.makedirs(os.path.join(logs, task.name))

    try:
        print("Running job {} on assigned port {}".format(job_id, context.port))
        print("Running early setup")
        for task in early_setup_tasks:
            task(context)
        print("Running setup")
        with context.set_log(os.path.join(logs, "log")):
            for task in setup_tasks:
                task(context)
                context.log.flush()
        print("Running tasks")
        skip = False
        for task in manifest.tasks:
            job_task = next(t for t in job.tasks if t.name == task.name)
            if skip:
                print("Skipping " + task.name)
                job_task.status = TaskStatus.skipped
                db.session.commit()
                continue
            print("Running " + task.name)
            job_task.status = TaskStatus.running
            db.session.commit()
            with open(os.path.join(logs, task.name, "log"), "wb") as f:
                r = context.ssh("./.tasks/" + task.name,
                        stdout=f, stderr=subprocess.STDOUT)
            if r.returncode == 255:
                skip = True
            elif r.returncode != 0:
                job_task.status = TaskStatus.failed
                db.session.commit()
                raise Exception("Task failed: {}".format(task.name))
            job_task.status = TaskStatus.success
            db.session.commit()
        job.status = JobStatus.success
        process_triggers(manifest, job)
        db.session.commit()
    except Exception as ex:
        job.status = JobStatus.failed
        process_triggers(manifest, job)
        db.session.commit()
        print(ex)
        raise ex
    finally:
        print("Cleaning up VM")
        control_cmd = cfg("builds.sr.ht::worker", "controlcmd")
        subprocess.run(shlex.split(control_cmd) + [
            manifest.image, "cleanup", str(context.port)
        ])
    print("Build complete.")
