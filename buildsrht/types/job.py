import sqlalchemy as sa
import sqlalchemy_utils as sau
from srht.database import Base
from buildsrht.manifest import Manifest
from enum import Enum

class JobStatus(Enum):
    pending = 'pending'
    queued = 'queued'
    running = 'running'
    success = 'success'
    failed = 'failed'

class Job(Base):
    __tablename__ = 'job'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.Unicode(256), nullable=False)
    _manifest = sa.Column(sa.Unicode(16384), nullable=False, name="manifest")
    owner_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'), nullable=False)
    owner = sa.orm.relationship('User', backref=sa.orm.backref('jobs'))
    job_group_id = sa.Column(sa.Integer, sa.ForeignKey('job_group.id'))
    job_group = sa.orm.relationship('JobGroup', backref=sa.orm.backref('jobs'))
    note = sa.Column(sa.Unicode(4096))
    runner = sa.Column(sa.String)
    status = sa.Column(
            sau.ChoiceType(JobStatus, impl=sa.String()),
            nullable=False,
            default=JobStatus.pending)

    @property
    def manifest(self):
        return self._manifest

    @manifest.setter
    def manifest_set(self, value):
        m = Manifest(value)
        self._manifest = value
