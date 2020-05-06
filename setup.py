#!/usr/bin/env python3
from distutils.core import setup
import subprocess
import os
import sys
import importlib.resources

with importlib.resources.path('srht', 'Makefile') as f:
    srht_path = f.parent.as_posix()

make = os.environ.get("MAKE", "make")
subp = subprocess.run([make, "SRHT_PATH=" + srht_path])
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
      'buildsrht-initdb',
      'buildsrht-keys',
      'buildsrht-migrate',
      'buildsrht-shell',
  ]
)
