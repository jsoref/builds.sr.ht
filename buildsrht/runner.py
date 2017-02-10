from srht.config import cfg, load_config, loaded
if not loaded():
    load_config("builds")

from srht.database import DbSession, db
if not hasattr(db, "session"):
    db = DbSession(cfg("sr.ht", "connection-string"))
    from buildsrht.types import Build
    db.init()
from buildsrht.types import Build

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

redis = Redis() # local redis
runner = Celery('builds', broker=cfg("builds.sr.ht", "redis"))
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
        "ssh", "-p", port,
        "-o", "UserKnownHostsFile=/dev/null",
        "-o", "StrictHostKeyChecking=no",
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

@runner.task
def run_build(build_id):
    build = Build.query.filter(Build.id == build_id).first()
    if not build:
        print("Error - no build by that ID")
        return
    manifest = Manifest(build.manifest)
    logs = os.path.join(buildlogs, str(build.job_id), str(build.id))
    os.makedirs(logs)
    with TemporaryDirectory(prefix="sr.ht.build.") as buildroot:
        root = TemporaryDirectory(prefix="sr.ht.").name
        print("Running build in ", buildroot)
        port = None
        try:
            run_or_die("sudo", os.path.join(images, "control"),
                manifest.image, "prepare", buildroot)
            root = os.path.join(buildroot, "temp", "root")
            home = os.path.join(root, "home", "build")

            os.makedirs(os.path.join(home, ".tasks"))
            for task in manifest.tasks:
                path = os.path.join(home, ".tasks", task.name)
                with open(path, "w") as f:
                    f.write("#!/usr/bin/env bash\n")
                    if manifest.environment:
                        f.write(". ~/.buildenv\n")
                    if not task.encrypted:
                        f.write("set -x\nset -e\n")
                    f.write(task.script)
                os.chmod(path, 0o755)

            if manifest.environment:
                write_env(manifest.environment, os.path.join(home, ".buildenv"))

            port = str(get_next_port())
            print("Booting image and waiting for it to settle")
            qemu = subprocess.Popen([
                "sudo", os.path.join(images, "control"),
                manifest.image, "boot", buildroot, port
            ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            time.sleep(5)
            if qemu.poll() != None:
                raise Exception("qemu aborted suspiciously early")

            print("Running sanity check")
            result = ssh(port, "echo", "hello world", stdout=subprocess.PIPE)
            if result.returncode != 0 or result.stdout != b"hello world\n":
                raise Exception("Sanity check failed, aborting build")

            print("Installing packages")
            if any(manifest.packages):
                with open(os.path.join(logs, ".packages.log"), "wb") as f:
                    r = run_or_die("sudo", os.path.join(images, "control"),
                        manifest.image, "install", port, *manifest.packages,
                        stdout=f, stderr=subprocess.STDOUT)

            print("Cloning repositories")
            for repo in manifest.repos:
                result = ssh(port, "git", "clone", "--recursive", repo)
                if result.returncode != 0:
                    raise Exception("git clone failed for {}".format(repo))

            print("Running tasks")
            for task in manifest.tasks:
                print("Running " + task.name)
                with open(os.path.join(logs, task.name + ".log"), "wb") as f:
                    r = ssh(port, "./.tasks/" + task.name,
                            stdout=f, stderr=subprocess.STDOUT)
                if r.returncode != 0:
                    raise Exception("Task failed: {}".format(task.name))

            print("Build complete.")
        except Exception as ex:
            print(ex)
            raise ex
        finally:
            subprocess.run([
                "sudo", os.path.join(images, "control"),
                manifest.image, "cleanup", buildroot
            ] + [port] if port else [])
