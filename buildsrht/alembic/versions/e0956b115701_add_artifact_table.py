"""Add artifact table

Revision ID: e0956b115701
Revises: e3d64733abb8
Create Date: 2020-02-25 14:58:28.878580

"""

# revision identifiers, used by Alembic.
revision = 'e0956b115701'
down_revision = 'e3d64733abb8'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_table("artifact",
        sa.Column("id", sa.Integer, primary_key=True),
        sa.Column("created", sa.DateTime, nullable=False),
        sa.Column("job_id", sa.Integer, sa.ForeignKey('job.id'), nullable=False),
        sa.Column("name", sa.Unicode, nullable=False),
        sa.Column("path", sa.Unicode, nullable=False),
        sa.Column("url", sa.Unicode, nullable=False),
        sa.Column("size", sa.Integer, nullable=False))


def downgrade():
    op.drop_table("artifact")
