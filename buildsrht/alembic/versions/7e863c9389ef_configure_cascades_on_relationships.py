"""Configure cascades on relationships

Revision ID: 7e863c9389ef
Revises: 6e5389a7ff68
Create Date: 2022-11-01 15:31:46.416272

"""

# revision identifiers, used by Alembic.
revision = '7e863c9389ef'
down_revision = '6e5389a7ff68'

from alembic import op
import sqlalchemy as sa

cascades = [
    ("secret", "user", "user_id", "CASCADE"),
    ("job_group", "user", "owner_id", "CASCADE"),
    ("job", "user", "owner_id", "CASCADE"),
    ("job", "job_group", "job_group_id", "SET NULL"),
    ("artifact", "job", "job_id", "CASCADE"),
    ("task", "job", "job_id", "CASCADE"),
    ("trigger", "job", "job_id", "CASCADE"),
    ("trigger", "job_group", "job_group_id", "CASCADE"),
    ("gql_user_wh_sub", "user", "user_id", "CASCADE"),
    ("oauthtoken", "user", "user_id", "CASCADE"),
]

def upgrade():
    for (table, relation, col, do) in cascades:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT IF EXISTS {table}_{col}_fkey;
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey
            FOREIGN KEY ({col})
            REFERENCES "{relation}"(id) ON DELETE {do};
        """)


def downgrade():
    for (table, relation, col, do) in tables:
        op.execute(f"""
        ALTER TABLE {table} DROP CONSTRAINT IF EXISTS {table}_{col}_fkey;
        ALTER TABLE {table} ADD CONSTRAINT {table}_{col}_fkey FOREIGN KEY ({col}) REFERENCES "{relation}"(id);
        """)
