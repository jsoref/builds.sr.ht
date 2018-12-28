"""Add user_type to user

Revision ID: c1f1264c4710
Revises: 279d3cd44391
Create Date: 2018-12-28 12:15:13.237296

"""

# revision identifiers, used by Alembic.
revision = 'c1f1264c4710'
down_revision = '279d3cd44391'

from alembic import op
import sqlalchemy as sa
import sqlalchemy_utils as sau
from enum import Enum
from sqlalchemy.ext.declarative import declarative_base
from sqlalchemy.orm import sessionmaker, Session as BaseSession, relationship
from srht.config import cfg
import requests
try:
    from tqdm import tqdm
except ImportError:
    def tqdm(iterable):
        yield from iterable

metasrht = cfg("meta.sr.ht", "origin")

Session = sessionmaker()
Base = declarative_base()

class UserType(Enum):
    unconfirmed = "unconfirmed"
    active_non_paying = "active_non_paying"
    active_free = "active_free"
    active_paying = "active_paying"
    active_delinquent = "active_delinquent"
    admin = "admin"

class User(Base):
    __tablename__ = 'user'
    id = sa.Column(sa.Integer, primary_key=True)
    username = sa.Column(sa.Unicode(256))
    oauth_token = sa.Column(sa.String(256), nullable=False)
    user_type = sa.Column(
            sau.ChoiceType(UserType, impl=sa.String()),
            nullable=False,
            default=UserType.unconfirmed)

def upgrade():
    op.drop_column('user', 'paid')
    op.add_column('user', sa.Column('user_type', sa.Unicode,
        nullable=False, server_default='active_non_paying'))

    bind = op.get_bind()
    session = Session(bind=bind)
    print("Migrating user_type (this expects meta.sr.ht to be available)")
    for user in tqdm(session.query(User).all()):
        r = requests.get("{}/api/user/profile".format(metasrht), headers={
            "Authorization": f"token {user.oauth_token}"
        })
        if r.status_code != 200:
            print(f"Failed for {user.username}", r.status_code, r.json())
            continue
        p = r.json()
        user.user_type = UserType(p["user_type"])
    session.commit()

def downgrade():
    op.drop_column('user', 'user_type')
    op.add_column('user', sa.Column('paid', sa.Boolean,
        nullable=False, server_default='f'))
