import sqlalchemy as sa
import sqlalchemy_utils as sau
from srht.database import Base
from buildsrht.manifest import Manifest

class Job(Base):
    __tablename__ = 'job'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.Unicode(256), nullable=False)
    _manifest = sa.Column(sa.Unicode(16384), nullable=False, name="manifest")
    owner_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'), nullable=False)
    owner = sa.orm.relationship('User', backref=sa.orm.backref('jobs'))

    @property
    def manifest(self):
        return self._manifest

    @manifest.setter
    def manifest_set(self, value):
        m = Manifest(value)
        self._manifest = value
