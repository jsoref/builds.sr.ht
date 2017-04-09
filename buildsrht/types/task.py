import sqlalchemy as sa
import sqlalchemy_utils as sau
from srht.database import Base
from enum import Enum

class TaskStatus(Enum):
    pending = 'pending'
    running = 'running'
    success = 'success'
    failed = 'failed'

class Task(Base):
    __tablename__ = 'task'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    name = sa.Column(sa.Unicode(256), nullable=False)
    status = sa.Column(
            sau.ChoiceType(TaskStatus, impl=sa.String()),
            nullable=False,
            default=TaskStatus.pending)
