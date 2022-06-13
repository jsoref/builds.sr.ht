"""Add GraphQL user webhook tables

Revision ID: f79186791a25
Revises: c0dd67b07f2b
Create Date: 2022-06-13 13:17:54.559113

"""

# revision identifiers, used by Alembic.
revision = 'f79186791a25'
down_revision = 'c0dd67b07f2b'

from alembic import op
import sqlalchemy as sa


def upgrade():
    op.execute("""
    CREATE TYPE webhook_event AS ENUM (
        'JOB_CREATED'
    );

    CREATE TYPE auth_method AS ENUM (
        'OAUTH_LEGACY',
        'OAUTH2',
        'COOKIE',
        'INTERNAL',
        'WEBHOOK'
    );

    CREATE TABLE gql_user_wh_sub (
        id serial PRIMARY KEY,
        created timestamp NOT NULL,
        events webhook_event[] NOT NULL check (array_length(events, 1) > 0),
        url varchar NOT NULL,
        query varchar NOT NULL,

        auth_method auth_method NOT NULL check (auth_method in ('OAUTH2', 'INTERNAL')),
        token_hash varchar(128) check ((auth_method = 'OAUTH2') = (token_hash IS NOT NULL)),
        grants varchar,
        client_id uuid,
        expires timestamp check ((auth_method = 'OAUTH2') = (expires IS NOT NULL)),
        node_id varchar check ((auth_method = 'INTERNAL') = (node_id IS NOT NULL)),

        user_id integer NOT NULL references "user"(id)
    );

    CREATE INDEX gql_user_wh_sub_token_hash_idx ON gql_user_wh_sub (token_hash);

    CREATE TABLE gql_user_wh_delivery (
        id serial PRIMARY KEY,
        uuid uuid NOT NULL,
        date timestamp NOT NULL,
        event webhook_event NOT NULL,
        subscription_id integer NOT NULL references gql_user_wh_sub(id) ON DELETE CASCADE,
        request_body varchar NOT NULL,
        response_body varchar,
        response_headers varchar,
        response_status integer
    );
    """)


def downgrade():
    op.execute("""
    DROP TABLE gql_user_wh_delivery;
    DROP INDEX gql_user_wh_sub_token_hash_idx;
    DROP TABLE gql_user_wh_sub;
    DROP TYPE auth_method;
    DROP TYPE webhook_event;
    """)
