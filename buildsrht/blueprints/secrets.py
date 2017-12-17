from flask import Blueprint, render_template, request, redirect, session
from flask_login import current_user
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import serialization
import pgpy

from srht.database import db
from srht.validation import Validation
from buildsrht.decorators import loginrequired
from buildsrht.types import Secret, SecretType

secrets = Blueprint('secrets', __name__)

@secrets.route("/secrets")
@loginrequired
def secrets_GET():
    message = session.get("message")
    if message:
        del session["message"]
    # TODO: Pagination I guess
    secrets = Secret.query.filter(Secret.user_id == current_user.id).all()
    return render_template("secrets.html", message=message, secrets=secrets)

@secrets.route("/secrets", methods=["POST"])
@loginrequired
def secrets_POST():
    valid = Validation(request)

    name = valid.optional("name")
    _secret = valid.require("secret", friendly_name="Secret")
    secret_type = valid.require("secret_type", friendly_name="Secret Type")
    valid.expect(not name or 3 < len(name) < 512,
            "Name must be between 3 and 512 characters", field="name")
    if secret_type is not None:
        try:
            secret_type = SecretType(secret_type)
        except:
            valid.error("{} is not a valid secret type".format(secret_type),
                    field="secret_type")

    if not valid.ok:
        return render_template("secrets.html", **valid.kwargs)

    secret = Secret(current_user, secret_type)

    if secret_type == SecretType.ssh_key:
        try:
            serialization.load_pem_private_key(
                    _secret.encode(),
                    password=None,
                    backend=default_backend())
        except Exception as ex:
            valid.error("Unable to load SSH key. Does it have a password?",
                    field="secret")
    elif secret_type == SecretType.pgp_key:
        try:
            key, _ = pgpy.PGPKey.from_blob(_secret.encode())
            if key.is_protected:
                valid.error("PGP key cannot be passphrase protected.",
                        field="secret")
        except Exception as ex:
            valid.error("Invalid PGP key.",
                    field="secret")
    elif secret_type == SecretType.plaintext_file:
        file_path = valid.require("file-path", friendly_name="Path")
        file_mode = valid.require("file-mode", friendly_name="Mode")
        if not valid.ok:
            return render_template("secrets.html", **valid.kwargs)
        try:
            file_mode = int(file_mode, 8)
        except:
            valid.error("Must be specified in octal",
                    field="file-mode")
        if not valid.ok:
            return render_template("secrets.html", **valid.kwargs)
        secret.path = file_path
        secret.mode = file_mode
    if not valid.ok:
        return render_template("secrets.html", **valid.kwargs)

    secret.name = name
    secret.secret = _secret

    db.session.add(secret)
    db.session.commit()

    session["message"] = "Successfully added secret {}.".format(secret.uuid)
    return redirect("/secrets")
