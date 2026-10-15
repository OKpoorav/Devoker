from datetime import datetime

from sqlalchemy import JSON, DateTime, ForeignKey, Integer, String, func
from sqlalchemy.dialects.postgresql import JSONB
from sqlalchemy.orm import Mapped, mapped_column

from .db import Base

JsonDoc = JSON().with_variant(JSONB(), "postgresql")


class RoomRow(Base):
    """One poker table. The whole game (deck, hole cards, tokens) is one JSON document, written under
    a row lock, so any backend replica can serve any request."""

    __tablename__ = "rooms"
    code: Mapped[str] = mapped_column(String(5), primary_key=True)
    state: Mapped[dict] = mapped_column(JsonDoc)
    version: Mapped[int] = mapped_column(Integer, default=1)  # bumped on every change; event streams watch it
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now(), onupdate=func.now())


class HandRow(Base):
    """A finished hand, for the table's history."""

    __tablename__ = "hands"
    id: Mapped[int] = mapped_column(Integer, primary_key=True, autoincrement=True)
    room_code: Mapped[str] = mapped_column(ForeignKey("rooms.code", ondelete="CASCADE"), index=True)
    number: Mapped[int] = mapped_column(Integer)
    pot: Mapped[int] = mapped_column(Integer)
    board: Mapped[list] = mapped_column(JsonDoc)
    winners: Mapped[list] = mapped_column(JsonDoc)
    finished_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())
