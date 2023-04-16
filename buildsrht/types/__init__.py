from srht.database import Base
from srht.oauth import ExternalUserMixin, ExternalOAuthTokenMixin

class User(Base, ExternalUserMixin):
    pass

class OAuthToken(Base, ExternalOAuthTokenMixin):
    pass

from .job import Job, JobStatus, Visibility
from .task import Task, TaskStatus
from .job_group import JobGroup
from .trigger import Trigger, TriggerType, TriggerCondition
from .secret import Secret, SecretType
from .artifact import Artifact
