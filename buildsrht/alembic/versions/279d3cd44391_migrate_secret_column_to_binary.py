"""migrate secret column to binary

Revision ID: 279d3cd44391
Revises: a8409937a19b
Create Date: 2018-08-28 23:28:52.933152

"""

# revision identifiers, used by Alembic.
revision = '279d3cd44391'
down_revision = 'a8409937a19b'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.alter_column(
        table_name='secret',
        column_name='secret',
        type_=sa.LargeBinary(16384),
        postgresql_using="convert_to(secret, 'UTF-8')"
    )

def downgrade():
    op.alter_column(
        table_name='secret',
        column_name='secret',
        type_=sa.Unicode(16384),
        postgresql_using="encode(secret, 'escape')"
    )
