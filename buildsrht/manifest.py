from srht.config import cfg
import subprocess
import pgpy
import re

_pgp_key, _ = pgpy.PGPKey.from_file(cfg("builds.sr.ht", "pgp-private-key"))

class Task():
    def __init__(self, yml):
        if not isinstance(yml, dict) or len(yml) != 1:
            raise Exception("Expected task to be a string: string")
        for key in yml:
            if not isinstance(key, str) or not isinstance(yml[key], str):
                raise Exception("Expected task to be a string: string")
            self.name = key
            self.script = yml[key].strip()
        if self.script.startswith("-----BEGIN PGP MESSAGE-----") \
                and self.script.endswith("-----END PGP MESSAGE-----"):
            # TODO: https://github.com/SecurityInnovation/PGPy/issues/160
            res = subprocess.run(["gpg", "--decrypt"],
                    input=self.script.encode(),
                    stdout=subprocess.PIPE,
                    stderr=subprocess.DEVNULL)
            if res.returncode != 0:
                raise Exception("Failed to decrypt encrypted script")
            self.script = res.stdout.decode()
            self.encrypted = True
        else:
            self.encrypted = False
        if not re.match(r"^[a-z0-9_]+$", self.name):
            raise Exception("Task name '{}' is invalid (must be all lowercase letters, numbers, and underscores)")

    def __repr__(self):
        return "<Task {}>".format(self.name)

class Manifest():
    def __init__(self, yml):
        self.yaml = yml
        image = yml.get("image")
        packages = yml.get("packages")
        repos = yml.get("repos")
        env = yml.get("environment")
        if not image:
            raise Exception("Missing image in manifest")
        if not isinstance(image, str):
            raise Exception("Expected imagease to be a string")
        if packages:
            if not isinstance(packages, list) or not all([isinstance(p, str) for p in packages]):
                raise Exception("Expected packages to be a string array")
        if repos:
            if not isinstance(repos, list) or not all([isinstance(r, str) for r in repos]):
                raise Exception("Expected repos to be a string array")
        if env:
            if not isinstance(env, dict):
                raise Exception("Expected environment to be a dictionary")
        self.image = image
        self.packages = packages
        self.repos = repos
        self.environment = env
        tasks = yml.get("tasks")
        if not tasks or not isinstance(tasks, list):
            raise Exception("Attempted to create manifest with no tasks")
        self.tasks = [Task(t) for t in tasks]
        for task in self.tasks:
            if len([t for t in self.tasks if t.name == task.name]) != 1:
                raise Exception("Duplicate task '{}'", task.name)

    def __repr__(self):
        return "<Manifest {}, {} tasks>".format(self.image, len(self.tasks))
