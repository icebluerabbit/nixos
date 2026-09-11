-- How many entities answer to each alias.
--
-- This is the specificity every match is weighted by: an alias forty performers
-- share is a first name, one that names a single studio is an identification.
-- Computing it per lookup meant a correlated subquery per hit over tens of
-- thousands of alias rows, which made answering one announce cost five figures
-- of row reads. It changes only when the catalogue is crawled, so it is
-- computed then and read as a table.
CREATE TABLE IF NOT EXISTS alias_freq (
    endpoint   TEXT    NOT NULL,
    kind       TEXT    NOT NULL,
    normalised TEXT    NOT NULL,
    entities   INTEGER NOT NULL,
    PRIMARY KEY (endpoint, kind, normalised)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_alias_freq_lookup
    ON alias_freq (endpoint, normalised, kind);

-- Fill it from what is already stored, so matching is weighted correctly from
-- the first announce rather than from the first crawl after this.
INSERT OR REPLACE INTO alias_freq (endpoint, kind, normalised, entities)
SELECT endpoint, 'performer', normalised, count(DISTINCT performer_id) FROM (
    SELECT endpoint, normalised, performer_id FROM performer_aliases
    UNION
    SELECT endpoint, normalised, performer_id FROM scene_performers
) GROUP BY endpoint, normalised;

INSERT OR REPLACE INTO alias_freq (endpoint, kind, normalised, entities)
SELECT endpoint, 'studio', normalised, count(DISTINCT studio_id)
  FROM studio_aliases GROUP BY endpoint, normalised;

INSERT OR REPLACE INTO alias_freq (endpoint, kind, normalised, entities)
SELECT endpoint, 'scene', title_normalised, count(*)
  FROM scenes WHERE title_normalised <> '' GROUP BY endpoint, title_normalised;
