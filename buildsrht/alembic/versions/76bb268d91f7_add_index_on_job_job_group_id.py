"""add index on job.job_group_id

Revision ID: 76bb268d91f7
Revises: 7e863c9389ef
Create Date: 2022-12-02 14:58:35.947429

"""

# revision identifiers, used by Alembic.
revision = '76bb268d91f7'
down_revision = '7e863c9389ef'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE INDEX ix_job_job_group_id ON job USING btree (job_group_id);
    """)


def downgrade():
    op.execute("""
    DROP INDEX ix_job_job_group_id;
    """)
