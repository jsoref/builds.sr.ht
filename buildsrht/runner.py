from srht.config import cfg, load_config, loaded
if not loaded():
    load_config("builds")

from celery import Celery
from buildsrht.manifest import Manifest
from tempfile import TemporaryDirectory
from redis import Redis
import subprocess
import random
import time
import os

redis = Redis() # local redis
runner = Celery('builds', broker=cfg("builds.sr.ht", "redis"))
images = cfg("builds.sr.ht", "images")

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
        raise Exception("{} exited with {}", " ".join(args), r.returncode)
    return r

@runner.task
def build(yml):
    manifest = Manifest(yml)
    with TemporaryDirectory(prefix="sr.ht.build.") as buildroot:
        root = TemporaryDirectory(prefix="sr.ht.").name
        print("Running build in ", buildroot)
        port = None
        try:
            run_or_die("sudo", os.path.join(images, "control"),
                manifest.image, "prepare", buildroot)
            root = os.path.join(buildroot, "persist")
            home = os.path.join(root, "home", "build")

            os.makedirs(os.path.join(home, ".tasks"))
            for task in manifest.tasks:
                path = os.path.join(home, ".tasks", task.name)
                with open(path, "w") as f:
                    f.write("#!/usr/bin/env bash\n")
                    f.write(task.script)
                os.chmod(path, 0o755)

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

            print("Cloning repositories")
            for repo in manifest.repos:
                result = ssh(port, "git", "clone", repo)
                if result.returncode != 0:
                    raise Exception("git clone failed for {}".format(repo))

            tree = subprocess.run([ "tree", os.path.join(buildroot, "persist") ],
                    stdout=subprocess.PIPE)
            print(tree.stdout.decode())
        except Exception as ex:
            print(ex)
            raise ex
        finally:
            subprocess.run([
                "sudo", os.path.join(images, "control"),
                manifest.image, "cleanup", buildroot
            ] + [port] if port else [])
