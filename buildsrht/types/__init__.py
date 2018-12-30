from srht.database import Base
from srht.oauth import ExternalUserMixin, ExternalOAuthTokenMixin

class User(Base, ExternalUserMixin):
    def __init__(*args, **kwargs):
        ExternalUserMixin.__init__(*args, **kwargs)

class OAuthToken(Base, ExternalOAuthTokenMixin):
    def __init__(*args, **kwargs):
        ExternalOAuthTokenMixin.__init__(*args, **kwargs)

from .job import Job, JobStatus
from .task import Task, TaskStatus
from .job_group import JobGroup
from .trigger import Trigger, TriggerType, TriggerCondition
from .secret import Secret, SecretType
