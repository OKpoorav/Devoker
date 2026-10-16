import { useEffect, useState } from 'react'

const SUITS = { Hearts: '♥', Diamonds: '♦', Clubs: '♣', Spades: '♠' }
const RANKS = { 14: 'A', 13: 'K', 12: 'Q', 11: 'J', 10: '10' }
const RANK_NAMES = { 14: 'Ace', 13: 'King', 12: 'Queen', 11: 'Jack' }

async function api(method, path, body) {
  const res = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || res.statusText)
  }
  return res.status === 204 ? null : res.json()
}

function loadSession() {
  try {
    return JSON.parse(localStorage.getItem('devoker'))
  } catch {
    return null
  }
}

export default function App() {
  const [session, setSession] = useState(loadSession)
  const save = (s) => {
    setSession(s)
    try {
      if (s) localStorage.setItem('devoker', JSON.stringify(s))
      else localStorage.removeItem('devoker')
    } catch {
      // private mode: the session just won't survive a reload
    }
  }
  return session ? <Room session={session} onLeave={() => save(null)} /> : <Lobby onJoin={save} />
}

function Lobby({ onJoin }) {
  const [name, setName] = useState('')
  const [code, setCode] = useState('')
  const [error, setError] = useState('')

  const enter = async (path) => {
    try {
      onJoin(await api('POST', path, { name }))
    } catch (e) {
      setError(e.message)
    }
  }

  return (
    <main className="lobby enter">
      <h1>Devoker v2</h1>
      <p className="tagline">Texas Hold'em with friends</p>
      <label>
        Your name
        <input value={name} maxLength={20} onChange={(e) => setName(e.target.value)} />
      </label>
      <button disabled={!name.trim()} onClick={() => enter('/api/rooms')}>
        Host a table
      </button>
      <div className="join">
        <input
          aria-label="Room code"
          placeholder="Room code"
          value={code}
          maxLength={5}
          onChange={(e) => setCode(e.target.value.toUpperCase())}
        />
        <button disabled={!name.trim() || code.length !== 5} onClick={() => enter(`/api/rooms/${code}/join`)}>
          Ask to join
        </button>
      </div>
      {error && <p className="error" role="alert">{error}</p>}
    </main>
  )
}

function Room({ session, onLeave }) {
  const [view, setView] = useState(null)
  const [error, setError] = useState('')
  const { room, token } = session

  useEffect(() => {
    const events = new EventSource(`/api/rooms/${room}/events?token=${encodeURIComponent(token)}`)
    events.onmessage = (e) => {
      const v = JSON.parse(e.data)
      setView(v)
      setError('')
      if (v.status === 'rejected' || v.status === 'closed') events.close()
    }
    events.onerror = () => setError('Connection lost. Reconnecting… (if the server restarted, leave and start a new table)')
    return () => events.close()
  }, [room, token])

  const send = (path, body = {}, method = 'POST') =>
    api(method, `/api/rooms/${room}/${path}`, { token, ...body }).catch((e) => setError(e.message))

  // Gives the seat back (or cancels the request) when possible; mid-hand the seat simply waits.
  const leave = () => {
    api('DELETE', `/api/rooms/${room}/seat`, { token }).catch(() => {})
    onLeave()
  }
  const closeTable = () => api('DELETE', `/api/rooms/${room}`, { token }).then(onLeave, (e) => setError(e.message))

  if (view && view.status !== 'seated') {
    const screens = {
      pending: ['Knock knock', `Waiting for the host of table ${room} to let you in…`, 'Cancel'],
      rejected: ['Not this time', `The host of table ${room} declined your request.`, 'Back'],
      closed: ['Table closed', `The host closed table ${room}.`, 'Back'],
    }
    const [title, text, button] = screens[view.status]
    return (
      <main className="lobby enter" key={view.status}>
        <h1>{title}</h1>
        <p className="tagline">{text}</p>
        <button className="secondary" onClick={view.status === 'pending' ? leave : onLeave}>
          {button}
        </button>
      </main>
    )
  }

  if (!view) {
    return (
      <main className="lobby">
        <p className="tagline">Connecting…</p>
        {error && <p className="error">{error}</p>}
        <button className="secondary" onClick={onLeave}>Leave</button>
      </main>
    )
  }

  return <Table view={view} room={room} send={send} error={error} onLeave={view.isOwner ? onLeave : leave} onClose={closeTable} />
}

function Table({ view, room, send, error, onLeave, onClose }) {
  const [raiseTo, setRaiseTo] = useState(view.minRaiseTo)
  const [lastMin, setLastMin] = useState(view.minRaiseTo)
  if (view.minRaiseTo !== lastMin) {
    // a new bet changed the minimum: reset the raise box to it
    setLastMin(view.minRaiseTo)
    setRaiseTo(view.minRaiseTo)
  }

  const nameOf = (id) => view.players.find((p) => p.id === id)?.name ?? '?'
  const me = view.players.find((p) => p.id === view.you)
  const shown = view.result?.shown ?? {}
  const myTurn = view.toAct === view.you
  const between = view.street === 'waiting'
  const canRebuy = me.chips === 0 && (between || !me.inHand)
  const funded = view.players.filter((p) => p.chips > 0).length

  return (
    <main className="table">
      <header className="material">
        <span>
          Table <b className="code">{room}</b> · blinds {view.smallBlind}/{view.bigBlind}
        </span>
        <span>
          {view.isOwner && <button className="link" onClick={onClose}>Close table</button>}
          <button className="link" onClick={onLeave}>Leave</button>
        </span>
      </header>

      {view.isOwner && view.requests.length > 0 && (
        <section className="requests" aria-label="Requests">
          {view.requests.map((r) => (
            <div className="request enter-down" key={r.id}>
              <span>
                <b>{r.name}</b> {r.kind === 'join' ? 'wants to join' : `wants to buy back in for ${view.startChips}`}
              </span>
              <span className="request-actions">
                <button className="secondary" onClick={() => send(`requests/${r.id}`, { approve: false }, 'PUT')}>Decline</button>
                <button onClick={() => send(`requests/${r.id}`, { approve: true }, 'PUT')}>Approve</button>
              </span>
            </div>
          ))}
        </section>
      )}

      <section className="felt">
        <div className="pot">
          Pot <span className="bump" key={view.pot}>{view.pot}</span>
        </div>
        <Cards cards={view.board} />
        {!between && <div className="street" key={view.street}>{view.street}</div>}
        {view.result && (
          <div className="result enter" role="status">
            {view.result.winners.map((w) => (
              <p key={w.playerId}>
                {nameOf(w.playerId)} wins {w.amount}
                {w.hand && ` with ${w.hand}`}
              </p>
            ))}
          </div>
        )}
      </section>

      <section className="players">
        {view.players.map((p) => {
          const net = p.chips - p.buyins * view.startChips
          return (
            <div
              key={p.id}
              className={['player', p.id === view.toAct && 'active', p.folded && 'folded', p.id === view.you && 'me']
                .filter(Boolean)
                .join(' ')}
            >
              <div className="name">
                {p.name}
                {p.id === view.dealer && <span className="dealer" title="Dealer">D</span>}
                {p.id === view.owner && <span className="tag host">host</span>}
              </div>
              <div className="chips">
                <span className="bump" key={p.chips}>{p.chips}</span>
              </div>
              <div className={net >= 0 ? 'net up' : 'net down'}>
                {net >= 0 ? '+' : '−'}
                {Math.abs(net)}
                {p.buyins > 1 && ` · ${p.buyins} buy-ins`}
              </div>
              {p.bet > 0 && <div className="bet">bet {p.bet}</div>}
              {p.allIn && !between && <div className="tag">all in</div>}
              {shown[p.id] && <Cards cards={shown[p.id]} small reveal />}
            </div>
          )
        })}
      </section>

      <section className="you material">
        <Cards cards={view.hole} />
        {canRebuy ? (
          view.rebuyPending ? (
            <p className="waiting">Buy-in requested. Waiting for the host…</p>
          ) : (
            <button onClick={() => send('rebuy')}>
              {view.isOwner ? `Buy back in for ${view.startChips}` : `Ask to buy back in for ${view.startChips}`}
            </button>
          )
        ) : between ? (
          <button onClick={() => send('start')} disabled={funded < 2}>
            {view.players.length < 2 ? 'Waiting for players… share the table code' : 'Deal next hand'}
          </button>
        ) : myTurn ? (
          <div className="actions enter-up" key={view.street + view.currentBet}>
            <button className="fold" onClick={() => send('action', { action: 'fold' })}>Fold</button>
            {view.callAmount === 0 ? (
              <button onClick={() => send('action', { action: 'check' })}>Check</button>
            ) : (
              <button onClick={() => send('action', { action: 'call' })}>Call {view.callAmount}</button>
            )}
            {view.maxRaiseTo > view.currentBet && (
              <>
                <input
                  type="number"
                  aria-label="Raise to"
                  min={view.minRaiseTo}
                  max={view.maxRaiseTo}
                  step={view.bigBlind}
                  value={raiseTo}
                  onChange={(e) => setRaiseTo(Number(e.target.value))}
                />
                <button onClick={() => send('action', { action: 'raise', amount: raiseTo })}>
                  {view.currentBet === 0 ? 'Bet' : 'Raise to'} {raiseTo}
                </button>
                <button onClick={() => send('action', { action: 'raise', amount: view.maxRaiseTo })}>All in</button>
              </>
            )}
          </div>
        ) : (
          <p className="waiting">
            {me.inHand ? `Waiting for ${nameOf(view.toAct)}…` : 'You join in from the next hand.'}
          </p>
        )}
        {error && <p className="error" role="alert">{error}</p>}
      </section>
    </main>
  )
}

function Cards({ cards, small, reveal }) {
  return (
    <div className={small ? 'cards small' : 'cards'}>
      {(cards ?? []).map((c, i) => (
        <span
          key={c.Value + c.Suit}
          style={{ '--i': i }}
          className={['card', reveal ? 'flip' : 'deal', (c.Suit === 'Hearts' || c.Suit === 'Diamonds') && 'red']
            .filter(Boolean)
            .join(' ')}
          aria-label={`${RANK_NAMES[c.Value] ?? c.Value} of ${c.Suit}`}
        >
          {RANKS[c.Value] ?? c.Value}
          {SUITS[c.Suit]}
        </span>
      ))}
    </div>
  )
}
