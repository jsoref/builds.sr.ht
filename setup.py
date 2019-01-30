#!/usr/bin/env python3
from distutils.core import setup
import subprocess
import os
import site
import sys

if hasattr(site, 'getsitepackages'):
    pkg_dirs = site.getsitepackages()
    if site.getusersitepackages():
        pkg_dirs.append(site.getusersitepackages())
    for pkg_dir in pkg_dirs:
        srht_path = os.path.join(pkg_dir, "srht")
        if os.path.isdir(srht_path):
            break
    else:
        raise Exception("Can't find core srht module in your site packages "
            "directories. Please install it first.")
else:
    srht_path = os.getenv("SRHT_PATH")
    if not srht_path:
        raise Exception("You're running inside a virtual environment. "
            "Due to virtualenv limitations, you need to set the "
            "$SRHT_PATH environment variable to the path of the "
            "core srht module.")
    elif not os.path.isdir(srht_path):
        raise Exception(
            "The $SRHT_PATH environment variable points to an invalid "
            "directory: {}".format(srht_path))

subp = subprocess.run(["make", "SRHT_PATH=" + srht_path])
if subp.returncode != 0:
    sys.exit(subp.returncode)

ver = os.environ.get("PKGVER") or subprocess.run(['git', 'describe', '--tags'],
      stdout=subprocess.PIPE).stdout.decode().strip()

setup(
  name = 'buildsrht',
  packages = [
      'buildsrht',
      'buildsrht.alembic',
      'buildsrht.alembic.versions',
      'buildsrht.blueprints',
      'buildsrht.types',
  ],
  version = ver,
  description = 'builds.sr.ht website',
  author = 'Drew DeVault',
  author_email = 'sir@cmpwn.com',
  url = 'https://git.sr.ht/~sircmpwn/builds.sr.ht',
  install_requires = [
      'srht',
      'redis',
      'celery',
      'flask_login',
      'pyyaml',
      'markdown',
      'bleach'
  ],
  license = 'AGPL-3.0',
  package_data={
      'buildsrht': [
          'templates/*.html',
          'static/*',
          'static/icons/*',
      ]
  },
  scripts = [
      'buildsrht-migrate'
  ]
)
