from srht.config import cfg, load_config, loaded
if not loaded():
    load_config("builds")

runner_name = None
from srht.database import DbSession, db
if not hasattr(db, "session"):
    db = DbSession(cfg("sr.ht", "connection-string"))
    import buildsrht.types
    db.init()
    runner_name = cfg("builds.sr.ht", "runner")

from buildsrht.types import Job, JobStatus, TaskStatus
from celery import Celery
from buildsrht.manifest import Manifest
from tempfile import TemporaryDirectory
from redis import Redis
import hashlib
import subprocess
import random
import time
import yaml
import os

runner = Celery('builds', broker=cfg("builds.sr.ht", "redis"))
if runner_name:
    redis = Redis() # local redis
    images = cfg("builds.sr.ht", "images")
    buildlogs = cfg("builds.sr.ht", "buildlogs")

def get_next_port():
    port = redis.incr("builds.sr.ht.ssh-port")
    if port < 22000:
        port = 22000
        redis.set("builds.sr.ht.ssh-port", port)
    if port >= 23000:
        port = 23000
        redis.set("builds.sr.ht.ssh-port", port)
    return port

def ssh(port, *args, **kwargs):
    return subprocess.run([
        "ssh", "-q", "-p", port,
        "-o", "UserKnownHostsFile=/dev/null",
        "-o", "StrictHostKeyChecking=no",
        "-o", "LogLevel=quiet",
        "build@localhost",
    ] + list(args), **kwargs)

def run_or_die(*args, **kwargs):
    print(" ".join(args))
    r = subprocess.run(args, **kwargs)
    if r.returncode != 0:
        raise Exception("{} exited with {}".format(" ".join(args), r.returncode))
    return r

def write_env(env, path):
    with open(path, "w") as f:
        for key in env:
            val = env[key]
            if isinstance(val, str):
                f.write("{}={}\n".format(key, val))
            elif isinstance(val, list):
                f.write("{}=({})\n".format(key,
                    " ".join(['"{}"'.format(v) for v in val])))
            else:
                print("Warning: unsupported env variable type")

def queue_build(job):
    job.status = JobStatus.queued
    db.session.commit()
    run_build.delay(job.id)

@runner.task
def run_build(job_id):
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        print("Error - no job by that ID")
        return
    job.runner = runner_name
    job.status = JobStatus.running
    db.session.commit()
    manifest = Manifest(yaml.load(job.manifest))
    logs = os.path.join(buildlogs, str(job.id))
    os.makedirs(logs)
    for task in manifest.tasks:
        os.makedirs(os.path.join(logs, task.name))
    print("Running job " + str(job_id))
    port = None
    try:
        port = str(get_next_port())
        print("Booting image and waiting for it to settle")
        qemu = subprocess.Popen([
            os.path.join(images, "control"),
            manifest.image, "boot", port
        ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(5)
        if qemu.poll() != None:
            raise Exception("qemu aborted suspiciously early")

        print("Running sanity check")
        result = ssh(port, "echo", "hello world", stdout=subprocess.PIPE)
        if result.returncode != 0 or result.stdout != b"hello world\n":
            raise Exception("Sanity check failed, aborting build")

        print("Sending build scripts")
        home = "/home/build"
        result = ssh(port, "mkdir", "-p", os.path.join(home, "/home/build/.tasks"))
        if result.returncode != 0:
            raise Exception("Failed to transfer scripts to build environment")
        for task in manifest.tasks:
            path = os.path.join(home, ".tasks", task.name)
            script = "#!/usr/bin/env bash\n"
            if manifest.environment:
                script += ". ~/.buildenv\n"
            if not task.encrypted:
                script += "set -x\nset -e\n"
            script += task.script
            ssh(port, "tee", path, input=script.encode(), stdout=subprocess.DEVNULL)
            ssh(port, "chmod", "755", path)

        if manifest.environment:
            write_env(manifest.environment, os.path.join(home, ".buildenv"))

        with open(os.path.join(logs, "log"), "wb") as f:
            print("Cloning repositories")
            for repo in manifest.repos:
                refname = None
                if "#" in repo:
                    _repo = repo.split("#")
                    refname = _repo[1]
                    repo = _repo[0]
                repo_name = os.path.basename(repo)
                result = ssh(port, "git", "clone", "--recursive", repo,
                    stdout=f, stderr=subprocess.STDOUT)
                if result.returncode != 0:
                    raise Exception("git clone failed for {}".format(repo))
                if refname:
                    _cmd = "'cd {} && git checkout {}'".format(repo_name, refname)
                    result = ssh(port, "sh", "-xc", _cmd,
                        stdout=f, stderr=subprocess.STDOUT)
                    if result.returncode != 0:
                        raise Exception("git checkout failed for {}#{}".format(
                            repo, refname))

            print("Installing packages")
            if any(manifest.packages):
                run_or_die(os.path.join(images, "control"),
                    manifest.image, "install", port, *manifest.packages,
                    stdout=f, stderr=subprocess.STDOUT)

        print("Running tasks")
        for task in manifest.tasks:
            print("Running " + task.name)
            job_task = next(t for t in job.tasks if t.name == task.name)
            job_task.status = TaskStatus.running
            db.session.commit()
            with open(os.path.join(logs, task.name, "log"), "wb") as f:
                r = ssh(port, "./.tasks/" + task.name,
                        stdout=f, stderr=subprocess.STDOUT)
            if r.returncode != 0:
                job_task.status = TaskStatus.failed
                db.session.commit()
                raise Exception("Task failed: {}".format(task.name))
            job_task.status = TaskStatus.success
            db.session.commit()

        job.status = JobStatus.success
        db.session.commit()
        print("Build complete.")
    except Exception as ex:
        job.status = JobStatus.failed
        db.session.commit()
        print(ex)
        raise ex
    finally:
        if port:
            subprocess.run([
                os.path.join(images, "control"),
                manifest.image, "cleanup", port
            ])
