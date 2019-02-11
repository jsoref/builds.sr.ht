"""Add image column to job table

Revision ID: e3d64733abb8
Revises: f6627d979e72
Create Date: 2019-02-11 13:16:42.993968

"""

# revision identifiers, used by Alembic.
revision = 'e3d64733abb8'
down_revision = 'f6627d979e72'

from alembic import op
import sqlalchemy as sa
from buildsrht.manifest import Manifest
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session as BaseSession, relationship
import yaml

Session = sessionmaker()
Base = declarative_base()

class Job(Base):
    __tablename__ = 'job'
    id = sa.Column(sa.Integer, primary_key=True)
    manifest = sa.Column(sa.Unicode(16384), nullable=False)
    image = sa.Column(sa.String(256))

def upgrade():
    op.add_column("job", sa.Column("image", sa.String(128)))
    bind = op.get_bind()
    session = Session(bind=bind)
    for job in session.query(Job).all():
        try:
            manifest = Manifest(yaml.safe_load(job.manifest))
            job.image = manifest.image
        except:
            continue
    session.commit()


def downgrade():
    op.delete_column("job", "image")
