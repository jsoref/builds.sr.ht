from srht.config import cfg, load_config, loaded
if not loaded():
    load_config("builds")

runner_name = None
from srht.database import DbSession, db
if not hasattr(db, "session"):
    runner_name = cfg("builds.sr.ht", "runner")
else:
    from buildsrht.types import Job, JobStatus, TaskStatus

from celery import Celery
from buildsrht.manifest import Manifest, TriggerAction, TriggerCondition
from tempfile import TemporaryDirectory
from redis import Redis
import hashlib
import subprocess
import random
import requests
import time
import yaml
import shlex
import os

buildenv = \
"""
#!/bin/sh
function complete-build() {
    exit 255
}
"""

control_cmd = cfg("builds.sr.ht", "controlcmd")

def init_db():
    db = DbSession(cfg("sr.ht", "connection-string"))
    import buildsrht.types
    db.init()

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

def write_env(port, env, path):
    script = buildenv[:]
    if env:
        for key in env:
            val = env[key]
            if isinstance(val, str):
                script += "{}={}\n".format(key, val)
            elif isinstance(val, list):
                script += "{}=({})\n".format(key,
                    " ".join(['"{}"'.format(v) for v in val]))
            else:
                print("Warning: unsupported env variable type")
    ssh(port, "tee", path, input=script.encode(), stdout=subprocess.DEVNULL)
    ssh(port, "chmod", "755", path)

def queue_build(job, manifest):
    job.status = JobStatus.queued
    db.session.commit()
    run_build.delay(job.id, manifest.to_dict())

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

@runner.task
def run_build(job_id, manifest):
    init_db()
    from buildsrht.types import Job, JobStatus, TaskStatus, Secret, SecretType
    job = Job.query.filter(Job.id == job_id).first()
    if not job:
        print("Error - no job by that ID")
        return
    job.runner = runner_name
    job.status = JobStatus.running
    db.session.commit()
    manifest = Manifest(manifest)
    logs = os.path.join(buildlogs, str(job.id))
    os.makedirs(logs)
    for task in manifest.tasks:
        os.makedirs(os.path.join(logs, task.name))
    print("Running job " + str(job_id))
    port = None
    try:
        port = str(get_next_port())
        print("Booting image and waiting for it to settle")
        print(shlex.split(control_cmd) + [
            manifest.image, "boot", port
        ])
        qemu = subprocess.Popen(shlex.split(control_cmd) + [
            manifest.image, "boot", port
        ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        time.sleep(5)
        if qemu.poll() != None:
            raise Exception("qemu aborted suspiciously early")

        print("Running sanity check")
        result = ssh(port, "echo", "hello world", stdout=subprocess.PIPE)
        if result.returncode != 0 or result.stdout != b"hello world\n":
            print(result.returncode, result.stdout)
            raise Exception("Sanity check failed, aborting build")
        
        print("Sending build scripts")
        home = "/home/build"
        result = ssh(port, "mkdir", "-p", os.path.join(home, "/home/build/.tasks"))
        if result.returncode != 0:
            raise Exception("Failed to transfer scripts to build environment")
        for task in manifest.tasks:
            path = os.path.join(home, ".tasks", task.name)
            script = "#!/usr/bin/env bash\n"
            script += ". ~/.buildenv\n"
            script += "set -x\nset -e\n"
            script += task.script
            ssh(port, "tee", path, input=script.encode(), stdout=subprocess.DEVNULL)
            ssh(port, "chmod", "755", path)

        write_env(port, manifest.environment, os.path.join(home, ".buildenv"))

        with open(os.path.join(logs, "log"), "wb") as f:
            ssh_key_used = False
            print("Resolving secrets")
            if manifest.secrets and any(manifest.secrets):
                for s in manifest.secrets:
                    secret = Secret.query.filter(Secret.uuid == s).first()
                    if not secret:
                        f.write("Warning: unknown secret {}\n".format(s).encode())
                        return
                    # TODO: more sophisticated checks here (i.e. orgs)
                    if secret.user_id != job.owner_id:
                        f.write("Warning: access denied for secret {}\n".format(s).encode())
                        return
                    if secret.secret_type == SecretType.ssh_key:
                        path = os.path.join("/home/build/.ssh", str(s))
                        ssh(port, "mkdir", "-p", "/home/build/.ssh",
                                stdout=subprocess.DEVNULL)
                        ssh(port, "tee", path,
                                input=secret.secret.encode(),
                                stdout=subprocess.DEVNULL)
                        ssh(port, "chmod", "600", path)
                        if not ssh_key_used:
                            ssh(port, "ln", "-s", str(s), "/home/build/.ssh/id_rsa")
                            ssh(port, "chmod", "600", "/home/build/.ssh/id_rsa")
                            ssh_key_used = True
                    elif secret.secret_type == SecretType.pgp_key:
                        # TODO: make this the default key similar to the SSH thing?
                        ssh(port, "gpg", "--import",
                                input=secret.secret.encode(),
                                stdout=f,
                                stderr=f)
                    elif secret.secret_type == SecretType.plaintext_file:
                        path = secret.path.replace("~", "/home/build")
                        ssh(port, "mkdir", "-p", os.path.dirname(path))
                        ssh(port, "tee", path,
                                input=secret.secret.encode(),
                                stdout=subprocess.DEVNULL)
                        ssh(port, "chmod", oct(secret.mode)[2:], path)
                    f.write("Loaded secret {}\n".format(str(s)).encode())
            f.flush()

            if manifest.repos and any(manifest.repos):
                print("Adding user repositories")
                for repo in manifest.repos:
                    source = manifest.repos[repo]
                    f.write("Adding repository: {}\n".format(repo).encode())
                    f.flush()
                    run_or_die(os.path.join(images, "control"), manifest.image,
                        "add-repo", port, repo, source, stdout=f, stderr=subprocess.STDOUT)

            if manifest.sources and any(manifest.sources):
                print("Cloning repositories")
                for repo in manifest.sources:
                    refname = None
                    if "#" in repo:
                        _repo = repo.split("#")
                        refname = _repo[1]
                        repo = _repo[0]
                    repo_name = os.path.basename(repo)
                    if repo_name.endswith(".git"):
                        repo_name = repo_name[:-4]
                    result = ssh(port, "git", "clone", "--recursive", repo,
                        stdout=f, stderr=subprocess.STDOUT)
                    if result.returncode != 0:
                        raise Exception("git clone failed for {}".format(repo))
                    if refname:
                        _cmd = "'cd {} && git checkout -q {}'".format(repo_name, refname)
                        result = ssh(port, "sh", "-xc", _cmd,
                            stdout=f, stderr=subprocess.STDOUT)
                        if result.returncode != 0:
                            raise Exception("git checkout failed for {}#{}".format(
                                repo, refname))

            if manifest.packages and any(manifest.packages):
                print("Installing packages")
                run_or_die(os.path.join(images, "control"),
                    manifest.image, "install", port, *manifest.packages,
                    stdout=f, stderr=subprocess.STDOUT)

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
                r = ssh(port, "./.tasks/" + task.name,
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
        print("Build complete.")
    except Exception as ex:
        job.status = JobStatus.failed
        process_triggers(manifest, job)
        db.session.commit()
        print(ex)
        raise ex
    finally:
        if port:
            subprocess.run(shlex.split(control_cmd) + [
                manifest.image, "cleanup", port
            ])
