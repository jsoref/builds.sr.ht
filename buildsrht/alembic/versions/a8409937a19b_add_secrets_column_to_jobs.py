"""Add secrets column to jobs

Revision ID: a8409937a19b
Revises: fbee7452ed5e
Create Date: 2018-03-11 20:47:02.697219

"""

# revision identifiers, used by Alembic.
revision = 'a8409937a19b'
down_revision = 'fbee7452ed5e'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('job', sa.Column('secrets',
        sa.Boolean(), nullable=False, server_default="t"))


def downgrade():
    op.drop_column('job', 'secrets')
