"""create rooms and hands

Revision ID: 0001
Revises:
Create Date: 2026-10-16
"""
import sqlalchemy as sa
from sqlalchemy.dialects import postgresql

from alembic import op

revision = "0001"
down_revision = None
branch_labels = None
depends_on = None

json_doc = sa.JSON().with_variant(postgresql.JSONB(), "postgresql")


def upgrade() -> None:
    op.create_table(
        "rooms",
        sa.Column("code", sa.String(5), primary_key=True),
        sa.Column("state", json_doc, nullable=False),
        sa.Column("version", sa.Integer, nullable=False, server_default="1"),
        sa.Column("created_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
        sa.Column("updated_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
    )
    op.create_table(
        "hands",
        sa.Column("id", sa.Integer, primary_key=True, autoincrement=True),
        sa.Column("room_code", sa.String(5), sa.ForeignKey("rooms.code", ondelete="CASCADE"), nullable=False),
        sa.Column("number", sa.Integer, nullable=False),
        sa.Column("pot", sa.Integer, nullable=False),
        sa.Column("board", json_doc, nullable=False),
        sa.Column("winners", json_doc, nullable=False),
        sa.Column("finished_at", sa.DateTime(timezone=True), server_default=sa.func.now(), nullable=False),
    )
    op.create_index("ix_hands_room_code", "hands", ["room_code"])


def downgrade() -> None:
    op.drop_index("ix_hands_room_code", "hands")
    op.drop_table("hands")
    op.drop_table("rooms")
