import sqlalchemy as sa
from datetime import datetime, timedelta
from srht.database import Base
import hashlib
import binascii
import os

class OAuthToken(Base):
    __tablename__ = 'oauthtoken'
    id = sa.Column(sa.Integer, primary_key=True)
    created = sa.Column(sa.DateTime, nullable=False)
    updated = sa.Column(sa.DateTime, nullable=False)
    expires = sa.Column(sa.DateTime, nullable=False)
    user_id = sa.Column(sa.Integer, sa.ForeignKey('user.id'))
    user = sa.orm.relationship('User', backref=sa.orm.backref('oauth_tokens'))
    token_hash = sa.Column(sa.String(128), nullable=False)
    token_partial = sa.Column(sa.String(8), nullable=False)
    scopes = sa.Column(sa.String(512), nullable=False)

    def __init__(self, user, token, expires):
        self.user_id = user.id
        self.expires = datetime.now() + timedelta(days=365)
        self.token_partial = token[:8]
        self.token_hash = hashlib.sha512(token.encode()).hexdigest()
