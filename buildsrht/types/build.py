import sqlalchemy as sa
import sqlalchemy_utils as sau
from srht.database import Base
from .buildtask import BuildTask

class Build(Base):
    __tablename__ = 'build'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    job_id = sa.Column(sa.Integer, sa.ForeignKey('job.id'))
    job = sa.orm.relationship('Job', backref=sa.orm.backref('builds'))
    owner_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'), nullable=False)
    owner = sa.orm.relationship('User', backref=sa.orm.backref('builds'))
    manifest = sa.Column(sa.Unicode(16384), nullable=False)
    source = sa.Column(sa.String(256))
    success = sa.Column(sa.Boolean)
    status = sa.Column(sa.Unicode(1024))

    def __init__(self, manifest):
        m = Manifest(manifest)
        self.manifest = manifest
        for task in m.tasks:
            bt = BuildTask(task.name)
            self.tasks.append(bt)
