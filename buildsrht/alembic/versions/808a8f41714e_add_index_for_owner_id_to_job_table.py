"""Add index for owner_id to Job table

Revision ID: 808a8f41714e
Revises: e0956b115701
Create Date: 2021-03-12 01:35:07.919372

"""

# revision identifiers, used by Alembic.
revision = '808a8f41714e'
down_revision = 'e0956b115701'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_index(op.f('ix_job_owner_id'), 'job', ['owner_id'], unique=False)


def downgrade():
    op.drop_index(op.f('ix_job_owner_id'), table_name='job')
