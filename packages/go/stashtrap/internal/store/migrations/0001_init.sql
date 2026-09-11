-- Announces, one row per release autobrr offered, with the decision made.
--
-- This is the corpus phase two designs matching against, so it keeps what the
-- tracker said rather than a normalised view of it: the questions are "what
-- does BitPorn actually send" and "what would this rule have matched", and
-- both need the original values.
CREATE TABLE announces (
    id                INTEGER PRIMARY KEY,
    received_at       TEXT    NOT NULL,

    name              TEXT    NOT NULL,
    indexer           TEXT    NOT NULL,
    -- Reported by autobrr, or computed from the torrent when it sent one.
    -- Empty when neither is available; not unique, because a tracker can
    -- re-announce and a cross-seed shares a hash across indexers.
    info_hash         TEXT    NOT NULL DEFAULT '',
    torrent_id        TEXT    NOT NULL DEFAULT '',
    category          TEXT    NOT NULL DEFAULT '',
    -- JSON array, kept as text: SQLite has json1 for querying it and there is
    -- no join that wants a tags table until a rule engine exists.
    tags              TEXT    NOT NULL DEFAULT '[]',
    release_tags      TEXT    NOT NULL DEFAULT '',
    size              INTEGER NOT NULL DEFAULT 0,
    freeleech         INTEGER NOT NULL DEFAULT 0,
    freeleech_percent INTEGER NOT NULL DEFAULT 0,
    uploader          TEXT    NOT NULL DEFAULT '',
    info_url          TEXT    NOT NULL DEFAULT '',

    -- Derived from the .torrent when the announce carried one. The raw bytes
    -- are not stored: piece hashes answer no question asked here.
    torrent_name      TEXT    NOT NULL DEFAULT '',
    -- JSON array of {path,length}.
    files             TEXT    NOT NULL DEFAULT '[]',
    file_count        INTEGER NOT NULL DEFAULT 0,
    total_size        INTEGER NOT NULL DEFAULT 0,
    private           INTEGER NOT NULL DEFAULT 0,
    source            TEXT    NOT NULL DEFAULT '',
    -- Set when the torrent failed to parse, so a template or tracker change
    -- shows up as data rather than as a gap.
    torrent_error     TEXT    NOT NULL DEFAULT '',

    accepted          INTEGER NOT NULL,
    reason            TEXT    NOT NULL
) STRICT;

CREATE INDEX announces_received_at ON announces (received_at);
CREATE INDEX announces_indexer     ON announces (indexer);
CREATE INDEX announces_info_hash   ON announces (info_hash);
