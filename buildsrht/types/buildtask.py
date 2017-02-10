import sqlalchemy as sa
import sqlalchemy_utils as sau
from srht.database import Base

class BuildTask(Base):
    __tablename__ = 'buildtask'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    build_id = sa.Column(sa.Integer, sa.ForeignKey('build.id'), nullable=False)
    build = sa.orm.relationship('Build', backref=sa.orm.backref('tasks'))
    name = sa.Column(sa.String(128), nullable=False)
    success = sa.Column(sa.Boolean)
    status = sa.Column(sa.Unicode(1024))

    def __init__(self, name):
        self.name = name
