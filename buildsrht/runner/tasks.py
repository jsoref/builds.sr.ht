from buildsrht.types import Secret, SecretType
from srht.config import cfg
from datetime import datetime, timedelta
import os
import shlex
import subprocess
import time

control_cmd = cfg("builds.sr.ht", "controlcmd")

def boot(ctx):
    print(shlex.split(control_cmd) + [
        ctx.manifest.image, "boot", str(ctx.port)
    ])
    qemu = subprocess.Popen(shlex.split(control_cmd) + [
        ctx.manifest.image, "boot", str(ctx.port)
    ], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    timeout = datetime.utcnow() + timedelta(seconds=60)
    check_passed = False
    while datetime.utcnow() < timeout and not check_passed:
        time.sleep(5)
        print("Running sanity check")
        result = ctx.ssh("echo", "hello world", stdout=subprocess.PIPE)
        if result.returncode == 0 and result.stdout == b"hello world\n":
            check_passed = True
            break
    if not check_passed:
        raise Exception("Sanity check failed, aborting build")

def send_tasks(ctx):
    print("Sending build scripts")
    home = "/home/build"
    result = ctx.ssh("mkdir", "-p", os.path.join(home, ".tasks"))
    if result.returncode != 0:
        raise Exception("Failed to transfer scripts to build environment")
    for task in ctx.manifest.tasks:
        path = os.path.join(home, ".tasks", task.name)
        script = "#!/usr/bin/env bash\n"
        script += ". ~/.buildenv\n"
        script += "set -x\nset -e\n"
        script += task.script
        ctx.ssh("tee", path,
                input=script.encode(),
                stdout=subprocess.DEVNULL)
        ctx.ssh("chmod", "755", path)

def write_env(ctx):
    home = "/home/build"
    path = os.path.join(home, ".buildenv")
    buildenv = \
"""
#!/bin/sh
function complete-build() {
    exit 255
}
"""
    script = buildenv[:]
    if ctx.manifest.environment:
        for key in ctx.manifest.environment:
            val = ctx.manifest.environment[key]
            if isinstance(val, str):
                script += "{}={}\n".format(key, val)
            elif isinstance(val, list):
                script += "{}=({})\n".format(key,
                    " ".join(['"{}"'.format(v) for v in val]))
            else:
                print("Warning: unsupported env variable type")
    ctx.ssh("tee", path, input=script.encode(), stdout=subprocess.DEVNULL)
    ctx.ssh("chmod", "755", path)

def resolve_secrets(ctx):
    if not ctx.job.secrets:
        print("Secrets disabled for this build")
        return
    if not ctx.manifest.secrets or not any(ctx.manifest.secrets):
        print("No secrets specified in manifest")
        return
    ssh_key_used = False
    print("Resolving secrets")
    for s in ctx.manifest.secrets:
        secret = Secret.query.filter(Secret.uuid == s).first()
        if not secret:
            ctx.log.write("Warning: unknown secret {}\n"
                    .format(s).encode())
            return
        # TODO: more sophisticated checks here (i.e. orgs)
        if secret.user_id != ctx.job.owner_id:
            ctx.log.write("Warning: access denied for secret {}\n"
                    .format(s).encode())
            return
        if secret.secret_type == SecretType.ssh_key:
            path = os.path.join("/home/build/.ssh", str(s))
            ctx.ssh("mkdir", "-p", "/home/build/.ssh",
                    stdout=subprocess.DEVNULL)
            ctx.ssh("tee", path,
                    input=secret.secret,
                    stdout=subprocess.DEVNULL)
            ctx.ssh("chmod", "600", path)
            if not ssh_key_used:
                ctx.ssh("ln", "-s", str(s), "/home/build/.ssh/id_rsa")
                ctx.ssh("chmod", "600", "/home/build/.ssh/id_rsa")
                ssh_key_used = True
        elif secret.secret_type == SecretType.pgp_key:
            # TODO: make this the default key similar to the SSH thing?
            ctx.ssh("gpg", "--import",
                    input=secret.secret,
                    stdout=ctx.log,
                    stderr=ctx.log)
        elif secret.secret_type == SecretType.plaintext_file:
            path = secret.path.replace("~", "/home/build")
            ctx.ssh("mkdir", "-p", os.path.dirname(path))
            ctx.ssh("tee", path,
                    input=secret.secret,
                    stdout=subprocess.DEVNULL)
            ctx.ssh("chmod", oct(secret.mode)[2:], path)
        ctx.log.write("Loaded secret {}\n".format(str(s)).encode())
    ctx.log.flush()

def configure_package_repos(ctx):
    if not ctx.manifest.repos or not any(ctx.manifest.repos):
        return
    print("Adding user repositories")
    for repo in ctx.manifest.repos:
        source = ctx.manifest.repos[repo]
        ctx.log.write("Adding repository: {}\n".format(repo).encode())
        ctx.log.flush()
        ctx.run_or_die(control_cmd, ctx.manifest.image,
                "add-repo", str(ctx.port), repo, source,
                stdout=ctx.log, stderr=subprocess.STDOUT)

def clone_git_repos(ctx):
    if not ctx.manifest.sources or not any(ctx.manifest.sources):
        return
    print("Cloning repositories")
    for repo in ctx.manifest.sources:
        refname = None
        if "#" in repo:
            _repo = repo.split("#")
            refname = _repo[1]
            repo = _repo[0]
        repo_name = os.path.basename(repo)
        if repo_name.endswith(".git"):
            repo_name = repo_name[:-4]
        print(repo)
        result = ctx.ssh("git", "clone",
                "--recursive",
                "--shallow-submodules",
                repo,
            stdout=ctx.log, stderr=subprocess.STDOUT)
        if result.returncode != 0:
            raise Exception("git clone failed for {}".format(repo))
        if refname:
            _cmd = "'cd {} && git checkout -q {}'".format(repo_name, refname)
            result = ctx.ssh("sh", "-xc", _cmd,
                stdout=ctx.log, stderr=subprocess.STDOUT)
            if result.returncode != 0:
                raise Exception("git checkout failed for {}#{}".format(
                    repo, refname))

def install_packages(ctx):
    if not ctx.manifest.packages or not any(ctx.manifest.packages):
        return
    print("Installing packages")
    ctx.run_or_die(control_cmd, ctx.manifest.image,
            "install", str(ctx.port), *ctx.manifest.packages,
            stdout=ctx.log, stderr=subprocess.STDOUT)

early_setup_tasks = [
    boot,
    send_tasks,
    write_env,
]

setup_tasks = [
    resolve_secrets,
    configure_package_repos,
    clone_git_repos,
    install_packages,
]
