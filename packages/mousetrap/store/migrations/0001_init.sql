-- sessions: current mam_id per named session (rotates every request → upserted, not history)
CREATE TABLE sessions (
  name       TEXT PRIMARY KEY,
  mam_id     TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

-- announces: every autobrr candidate + our decision
CREATE TABLE announces (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  ts              INTEGER NOT NULL,
  tid             INTEGER,
  name            TEXT,
  category        TEXT,
  kind            TEXT,
  filetype        TEXT,
  language        TEXT,
  size            INTEGER,
  vip             INTEGER,             -- 0/1 raw flag
  freeleech       INTEGER,             -- 0/1 raw flag
  accept_decision TEXT NOT NULL,       -- accept|reject
  accept_reason   TEXT NOT NULL,
  grab_decision   TEXT,                -- grab|hold|skip (null if L1 rejected)
  grab_reason     TEXT,
  payload         TEXT                 -- raw JSON, durable analytics
);

-- spends: our own optimistic outgoing spends (wedges + points). Source of in-flight
-- wedge accounting and spent metrics — recorded when WE spend, before MAM's
-- userBonusHistory reflects it.
CREATE TABLE spends (
  id       INTEGER PRIMARY KEY AUTOINCREMENT,
  ts       INTEGER NOT NULL,
  type     TEXT NOT NULL,     -- wedge|points
  amount   INTEGER,           -- points spent; wedges are 1 each
  tid      INTEGER,           -- torrent for a wedge-on-grab; NULL for shop
  reason   TEXT NOT NULL,     -- grab|shop_vip|shop_upload
  response TEXT               -- shop-buy outcome; NULL for a wedge-on-grab
);

-- grabs: every grab action + outcome. announce_id links to the originating announce
-- (NULL for torznab); spend_id links to the wedge spend, if any (NULL = no wedge).
CREATE TABLE grabs (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  ts          INTEGER NOT NULL,
  announce_id INTEGER REFERENCES announces(id),  -- NULL for torznab-originated
  spend_id    INTEGER REFERENCES spends(id),     -- NULL unless a wedge was spent
  source      TEXT NOT NULL,           -- announce|torznab
  tid         INTEGER NOT NULL,
  size        INTEGER,
  costs_ratio INTEGER NOT NULL,        -- 0/1
  result      TEXT NOT NULL,           -- GrabResult
  response    TEXT                     -- success/error response (status/body/err)
);

-- bonuses: full mirror of MAM userBonusHistory (income and MAM's own spend records).
-- Received/history metrics. amount is signed (negative = spent per MAM).
CREATE TABLE bonuses (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  ts           REAL NOT NULL,          -- MAM timestamp is fractional unix
  type         TEXT NOT NULL,          -- giftPoints|giftWedge|wedgePF|wedgeGFL|torrentThanks|millionaires
  amount       INTEGER,
  tid          INTEGER,
  other_userid INTEGER,
  other_name   TEXT
);

CREATE INDEX idx_grabs_tid    ON grabs(tid);      -- dedup
CREATE INDEX idx_grabs_ts     ON grabs(ts);       -- in-flight window (unsat/buffer)
CREATE INDEX idx_spends_ts    ON spends(ts);      -- in-flight window (wedges) + spent metrics
CREATE INDEX idx_bonuses_ts   ON bonuses(ts);     -- cursor = MAX(ts)
CREATE INDEX idx_announces_ts ON announces(ts);
