#!/usr/bin/env python3
from distutils.core import setup
import subprocess
import glob
import os

subprocess.call(["make"])

ver = os.environ.get("PKGVER") or subprocess.run(['git', 'describe', '--tags'],
      stdout=subprocess.PIPE).stdout.decode().strip()

setup(
  name = 'buildsrht',
  packages = [
      'buildsrht',
      'buildsrht.alembic',
      'buildsrht.alembic.versions',
      'buildsrht.blueprints',
      'buildsrht.runner',
      'buildsrht.types',
  ],
  version = ver,
  description = 'builds.sr.ht website and runner',
  author = 'Drew DeVault',
  author_email = 'sir@cmpwn.com',
  url = 'https://git.sr.ht/~sircmpwn/builds.sr.ht',
  install_requires = [
      'srht',
      'pgpy',
      'redis',
      'celery',
      'yaml',
      'markdown',
      'bleach'
  ],
  license = 'AGPL-3.0',
  package_data={
      'buildsrht': [
          'templates/*.html',
          'static/*',
      ]
  }
)
