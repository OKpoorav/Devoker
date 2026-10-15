from logging.config import fileConfig

from sqlalchemy import text

from alembic import context
from app import models  # noqa: F401  (registers the tables on Base.metadata)
from app.db import Base, engine

if context.config.config_file_name:
    fileConfig(context.config.config_file_name, disable_existing_loggers=False)

with engine.connect() as connection:
    if connection.dialect.name == "postgresql":
        # Every backend replica runs migrations on start. The lock makes them take turns; whoever
        # goes second finds the schema already at head and does nothing.
        connection.execute(text("SELECT pg_advisory_lock(2026)"))
        # The SELECT opened a transaction; end it so Alembic starts (and commits) its own. The lock
        # belongs to the connection, so it is held until the connection closes.
        connection.commit()
    context.configure(connection=connection, target_metadata=Base.metadata)
    with context.begin_transaction():
        context.run_migrations()
