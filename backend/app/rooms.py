"""Rooms: who sits at a poker table, who is waiting for the host, and how a room is loaded and
saved. Every change runs inside locked(), which holds the room's row lock until it commits."""

import copy
import secrets
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field

from sqlalchemy import select
from sqlalchemy.orm import Session

from .models import RoomRow
from .poker import Table


class ApiError(Exception):
    def __init__(self, status: int, message: str):
        self.status, self.message = status, message


@dataclass
class Room:
    table: Table
    owner: str  # player ID of the host, who approves joins and rebuys
    tokens: dict[str, str] = field(default_factory=dict)  # secret player token -> player ID, once seated
    requests: list[dict] = field(default_factory=list)  # joins and rebuys waiting for the host
    rejected: list[str] = field(default_factory=list)  # tokens of declined join requests
    hands_played: int = 0

    def seated(self, token: str) -> str:
        """The player ID behind a token, or 403."""
        if token not in self.tokens:
            raise ApiError(403, "you are not seated at this table")
        return self.tokens[token]

    def known(self, token: str) -> bool:
        return token in self.tokens or token in self.rejected or any(r.get("token") == token for r in self.requests)

    def view(self, token: str) -> dict:
        """What one player's event stream shows. Status is seated, pending or rejected; the table
        fields are only filled in once seated."""
        if token in self.tokens:
            pid = self.tokens[token]
            is_owner = pid == self.owner
            return {
                **self.table.view(pid), "status": "seated", "isOwner": is_owner, "owner": self.owner,
                # Only id, kind and name: the waiting player's token must never reach the host's browser.
                "requests": [{k: r[k] for k in ("id", "kind", "name")} for r in self.requests] if is_owner else [],
                "rebuyPending": any(r.get("playerId") == pid for r in self.requests),
            }
        return {"status": "rejected" if token in self.rejected else "pending"}

    def to_dict(self) -> dict:
        return {"table": self.table.to_dict(), "owner": self.owner, "tokens": self.tokens, "requests": self.requests,
                "rejected": self.rejected, "hands_played": self.hands_played}

    @staticmethod
    def from_dict(d: dict) -> "Room":
        return Room(**{**d, "table": Table.from_dict(d["table"])})


def new_token() -> str:
    return secrets.token_urlsafe(18)


def new_code() -> str:
    alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"  # no 0/O or 1/I to misread
    return "".join(secrets.choice(alphabet) for _ in range(5))


def load(db: Session, code: str, lock: bool = False) -> tuple[RoomRow, Room]:
    query = select(RoomRow).where(RoomRow.code == code.upper())
    row = db.execute(query.with_for_update() if lock else query).scalar_one_or_none()
    if row is None:
        raise ApiError(404, "room not found")
    # A deep copy: editing the room must not edit the loaded state in place, or SQLAlchemy sees no change to save.
    return row, Room.from_dict(copy.deepcopy(row.state))


@contextmanager
def locked(db: Session, code: str) -> Iterator[tuple[RoomRow, Room]]:
    """Loads a room under SELECT ... FOR UPDATE and saves it when the block finishes. Two replicas
    acting on the same table queue on the lock instead of overwriting each other."""
    row, room = load(db, code, lock=True)
    yield row, room
    row.state = room.to_dict()
    row.version += 1
    db.commit()
