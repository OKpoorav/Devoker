import os
import tempfile

# A throwaway SQLite file, never the real database. Set before the app is imported.
os.environ["DATABASE_URL"] = f"sqlite:///{tempfile.mkdtemp()}/test.db"

import pytest  # noqa: E402
from alembic.config import Config  # noqa: E402
from fastapi.testclient import TestClient  # noqa: E402
from sqlalchemy import delete  # noqa: E402

from alembic import command  # noqa: E402
from app.db import SessionLocal  # noqa: E402
from app.main import app  # noqa: E402
from app.models import HandRow, RoomRow  # noqa: E402


@pytest.fixture(scope="session", autouse=True)
def schema():
    command.upgrade(Config("alembic.ini"), "head")  # builds the test schema with the real migration


@pytest.fixture
def client():
    yield TestClient(app)
    with SessionLocal() as db:
        db.execute(delete(HandRow))
        db.execute(delete(RoomRow))
        db.commit()


class Table:
    """A room seen through the API: host plus any approved guests."""

    def __init__(self, client: TestClient):
        self.c = client
        r = client.post("/api/rooms", json={"name": "host"})
        assert r.status_code == 201
        self.code, self.host = r.json()["room"], r.json()["token"]

    def view(self, token: str) -> dict:
        r = self.c.get(f"/api/rooms/{self.code}", params={"token": token})
        assert r.status_code == 200, r.text
        return r.json()

    def ask(self, name: str) -> str:
        r = self.c.post(f"/api/rooms/{self.code}/join", json={"name": name})
        assert r.status_code == 201
        return r.json()["token"]

    def decide(self, token: str, request_id: str, approve: bool):
        return self.c.put(f"/api/rooms/{self.code}/requests/{request_id}", json={"token": token, "approve": approve})

    def seat(self, name: str) -> str:
        token = self.ask(name)
        req = self.view(self.host)["requests"][-1]
        assert self.decide(self.host, req["id"], True).status_code == 204
        return token

    def post(self, path: str, **body):
        return self.c.post(f"/api/rooms/{self.code}/{path}", json=body)

    def delete(self, path: str, token: str):
        return self.c.request("DELETE", f"/api/rooms/{self.code}{path}", json={"token": token})


@pytest.fixture
def table(client) -> Table:
    return Table(client)
