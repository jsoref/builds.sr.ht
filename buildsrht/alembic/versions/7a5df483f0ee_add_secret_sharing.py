"""Add secret sharing

Revision ID: 7a5df483f0ee
Revises: ae3544d6450a
Create Date: 2023-08-28 09:49:34.659942

"""

# revision identifiers, used by Alembic.
revision = '7a5df483f0ee'
down_revision = 'ae3544d6450a'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    ALTER TABLE secret
    ADD COLUMN from_user_id integer REFERENCES "user"(id) ON DELETE SET NULL;

    ALTER TABLE secret
    ADD CONSTRAINT secret_user_id_uuid_unique UNIQUE (user_id, uuid);
    """)


def downgrade():
    op.execute("""
    ALTER TABLE secret
    DROP COLUMN from_user_id;

    ALTER TABLE secret
    DROP CONSTRAINT secret_user_id_uuid_unique;
    """)
