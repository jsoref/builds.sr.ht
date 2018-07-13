from buildsrht.runner import redis
import subprocess

def _get_next_port():
    port = redis.incr("builds.sr.ht.ssh-port")
    if port < 22000:
        port = 22000
        redis.set("builds.sr.ht.ssh-port", port)
    if port >= 23000:
        port = 22000
        redis.set("builds.sr.ht.ssh-port", port)
    return port

class BuildContext:
    def __init__(self, job, manifest):
        self.job = job
        self.manifest = manifest
        self.port = _get_next_port()

    def ssh(self, *args, **kwargs):
        return subprocess.run([
            "ssh", "-q", "-p", str(self.port),
            "-o", "UserKnownHostsFile=/dev/null",
            "-o", "StrictHostKeyChecking=no",
            "-o", "LogLevel=quiet",
            "build@localhost",
        ] + list(args), **kwargs)

    def run_or_die(self, *args, **kwargs):
        print(" ".join(args))
        r = subprocess.run(args, **kwargs)
        if r.returncode != 0:
            raise Exception("{} exited with {}".format(
                " ".join(args), r.returncode))
        return r

    def set_log(self, path):
        self.log = open(path, "wb")
        return self.log
