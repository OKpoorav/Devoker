# Devoker

Texas Hold'em with friends: a Go backend and a React frontend.

## Run it locally

```bash
# terminal 1: backend on :8080
cd backend && go run ./cmd/server

# terminal 2: frontend on :5173 (proxies /api to :8080)
cd frontend && npm install && npm run dev
```

Open http://localhost:5173, create a table and share the 5-letter room code. Friends on the same
network can join with `npm run dev -- --host` and your machine's IP.

## Tests

```bash
cd backend && go test -cover ./...
```

## How it fits together

| Part | What it does |
|---|---|
| `backend/poker` | Cards, deck, hand evaluator, and the table (blinds, betting rounds, side pots, showdown). No networking. |
| `backend/cmd/server` | HTTP API. Each player streams their own view of the table over Server-Sent Events and sends actions with POST. |
| `frontend` | React app: lobby, table, action buttons. |

### API

| Method | Path | Body | Returns |
|---|---|---|---|
| GET | `/health` | | `ok` |
| POST | `/api/rooms` | `{"name"}` | `{"room","token"}` |
| POST | `/api/rooms/{code}/join` | `{"name"}` | `{"room","token"}` |
| POST | `/api/rooms/{code}/start` | `{"token"}` | 204 |
| POST | `/api/rooms/{code}/action` | `{"token","action":"fold\|check\|call\|raise","amount"}` | 204 |
| POST | `/api/rooms/{code}/rebuy` | `{"token"}` | 204 |
| POST | `/api/rooms/{code}/requests/{id}` | `{"token","approve"}` | 204 (host only) |
| GET | `/api/rooms/{code}/events?token=` | | SSE stream of the table as JSON |

The token is a secret per player. Only the owner's stream includes their hole cards.

Whoever creates the table is the **host**. Joining sends the host a request; the player's stream
reports `"status":"pending"` until the host approves (`"seated"`) or declines (`"rejected"`).
A busted player can buy back in between hands: the host's own rebuy is immediate, everyone
else's goes to the host for approval.

### Configuration (environment variables)

| Variable | Default |
|---|---|
| `PORT` | `8080` |
| `STARTING_CHIPS` | `1000` |
| `SMALL_BLIND` | `10` |
| `BIG_BLIND` | `20` |

## Things to know before deploying

- **Game state lives in the server's memory.** Run the backend as **1 replica**. With more, two
  friends can land on different pods and see different tables. Restarting the pod ends all games.
- Behind nginx or ingress-nginx, the event stream needs buffering off (the server sends
  `X-Accel-Buffering: no`) and a read timeout above 25s (the server pings every 25s).
- There is no turn timer: a player who closes their tab mid-hand stalls the table until they
  come back (their seat survives a reload).
