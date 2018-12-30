"""Add user mixin properties

Revision ID: f6627d979e72
Revises: c1f1264c4710
Create Date: 2018-12-30 13:28:01.936881

"""

# revision identifiers, used by Alembic.
revision = 'f6627d979e72'
down_revision = 'c1f1264c4710'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column("user", sa.Column("url", sa.String(256)))
    op.add_column("user", sa.Column("location", sa.Unicode(256)))
    op.add_column("user", sa.Column("bio", sa.Unicode(4096)))
    op.add_column("user", sa.Column("oauth_token_scopes",
        sa.String, nullable=False, default="profile", server_default="profile"))


def downgrade():
    op.delete_column("user", "url")
    op.delete_column("user", "location")
    op.delete_column("user", "bio")
    op.delete_column("user", "oauth_token_scopes")
