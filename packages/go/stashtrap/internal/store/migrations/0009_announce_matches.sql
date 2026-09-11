-- What the reading concluded about each announce.
--
-- Until now a row recorded only the decision and its reason, and everything
-- behind it — who was recognised, under which spelling, which scene it was
-- identified as — went to the logs. That is enough to watch the service and
-- not enough to answer anything afterwards: which releases named a followed
-- performer, why a particular grab happened, what would now match a performer
-- followed since. All of those are questions about stored rows, so the answers
-- belong beside them.
ALTER TABLE announces ADD COLUMN scene_id TEXT NOT NULL DEFAULT '';
ALTER TABLE announces ADD COLUMN score REAL NOT NULL DEFAULT 0;
ALTER TABLE announces ADD COLUMN corroborated INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS announce_matches (
    announce_id INTEGER NOT NULL,
    -- performer, studio or scene.
    kind      TEXT    NOT NULL,
    entity_id TEXT    NOT NULL,
    -- The text that matched, in the release's own spelling: a false positive
    -- is diagnosed from the words that caused it, not from an id.
    probe     TEXT    NOT NULL,
    -- Whether this entity is followed, and so whether it drove the decision.
    favourite INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (announce_id, kind, entity_id),
    FOREIGN KEY (announce_id) REFERENCES announces (id) ON DELETE CASCADE
) STRICT;

CREATE INDEX IF NOT EXISTS idx_announce_matches_entity
    ON announce_matches (kind, entity_id, favourite);

CREATE INDEX IF NOT EXISTS idx_announce_matches_favourite
    ON announce_matches (favourite, kind);
