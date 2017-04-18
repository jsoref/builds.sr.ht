from srht.config import cfg
import subprocess
import yaml
import pgpy
import re

_privkey_path = cfg("builds.sr.ht", "pgp-private-key", default=None)
if _privkey_path:
    _pgp_key, _ = pgpy.PGPKey.from_file(cfg("builds.sr.ht", "pgp-private-key"))
else:
    _pgp_key = None

class Task():
    def __init__(self, yml):
        if not isinstance(yml, dict) or len(yml) != 1:
            raise Exception("Expected task to be a string: string")
        for key in yml:
            if not isinstance(key, str) or not isinstance(yml[key], str):
                raise Exception("Expected task to be a string: string")
            self.name = key
            self.script = yml[key].strip()
        if _pgp_key and self.script.startswith("-----BEGIN PGP MESSAGE-----") \
                and self.script.endswith("-----END PGP MESSAGE-----"):
            # TODO: https://github.com/SecurityInnovation/PGPy/issues/160
            res = subprocess.run(["gpg", "--decrypt"],
                    input=self.script.encode(),
                    stdout=subprocess.PIPE,
                    stderr=subprocess.DEVNULL)
            if res.returncode != 0:
                raise Exception("Failed to decrypt encrypted script")
            self.encrypted_script = self.script
            self.script = res.stdout.decode()
            self.encrypted = True
        else:
            self.encrypted = False
        if not re.match(r"^[a-z0-9_]+$", self.name) or len(self.name) > 128:
            raise Exception("Task name '{}' is invalid (must be all lowercase letters, numbers, and underscores, and <=128 characters)")

    def __repr__(self):
        return "<Task {}>".format(self.name)

class Manifest():
    def __init__(self, yml):
        self.yaml = yml
        image = self.yaml.get("image")
        packages = self.yaml.get("packages")
        repos = self.yaml.get("repos")
        env = self.yaml.get("environment")
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
        tasks = self.yaml.get("tasks")
        if not tasks or not isinstance(tasks, list):
            raise Exception("Attempted to create manifest with no tasks")
        self.tasks = [Task(t) for t in tasks]
        for task in self.tasks:
            if len([t for t in self.tasks if t.name == task.name]) != 1:
                raise Exception("Duplicate task '{}'", task.name)

    def __repr__(self):
        return "<Manifest {}, {} tasks>".format(self.image, len(self.tasks))
    
    def to_dict(self, encrypted=True):
        return {
            "image": self.image,
            "packages": self.packages,
            "repos": self.repos,
            "environment": self.environment,
            "tasks": [{
                t.name: t.encrypted_script if t.encrypted and encrypted else t.script
            }] for t in self.tasks
        }

    def to_yaml(self):
        return yaml.dump(self.to_dict())
