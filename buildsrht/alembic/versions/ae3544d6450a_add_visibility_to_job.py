"""Add visibility to job

Revision ID: ae3544d6450a
Revises: 76bb268d91f7
Create Date: 2023-03-13 10:33:49.830104

"""

# revision identifiers, used by Alembic.
revision = 'ae3544d6450a'
down_revision = '76bb268d91f7'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE TYPE visibility AS ENUM (
        'PUBLIC',
        'UNLISTED',
        'PRIVATE'
    );

    ALTER TABLE job
    ADD COLUMN visibility visibility;

    UPDATE job
    SET visibility = 'UNLISTED'::visibility;

    ALTER TABLE job
    ALTER COLUMN visibility
    SET NOT NULL;
    """)


def downgrade():
    op.execute("""
    ALTER TABLE job DROP COLUMN visibility;
    DROP TYPE visibility;
    """)
