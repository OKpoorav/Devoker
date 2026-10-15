from typing import Annotated, Literal

from pydantic import BaseModel, StringConstraints

Name = Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=20)]


class NameIn(BaseModel):
    name: Name


class TokenIn(BaseModel):
    token: str


class ActionIn(TokenIn):
    action: Literal["fold", "check", "call", "raise"]
    amount: int = 0


class DecisionIn(TokenIn):
    approve: bool


class Joined(BaseModel):
    room: str
    token: str


class Hand(BaseModel):
    number: int
    pot: int
    board: list[dict]
    winners: list[dict]
