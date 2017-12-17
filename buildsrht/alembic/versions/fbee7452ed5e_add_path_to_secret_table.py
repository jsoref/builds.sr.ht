"""Add path to secret table

Revision ID: fbee7452ed5e
Revises: de86fd750e68
Create Date: 2017-12-17 14:37:03.154776

"""

# revision identifiers, used by Alembic.
revision = 'fbee7452ed5e'
down_revision = 'de86fd750e68'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.add_column('secret', sa.Column('path', sa.Unicode(512)))
    op.add_column('secret', sa.Column('mode', sa.Integer()))


def downgrade():
    op.drop_column('secret', 'path')
    op.drop_column('secret', 'mode')
