"""Add index for job_id to Task table

Revision ID: c0dd67b07f2b
Revises: 808a8f41714e
Create Date: 2021-03-15 17:11:42.153670

"""

# revision identifiers, used by Alembic.
revision = 'c0dd67b07f2b'
down_revision = '808a8f41714e'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.create_index(op.f('ix_task_job_id'), 'task', ['job_id'], unique=False)


def downgrade():
    op.drop_index(op.f('ix_task_job_id'), table_name='task')
