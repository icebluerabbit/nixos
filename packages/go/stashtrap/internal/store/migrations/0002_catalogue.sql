-- The stash-box catalogue: the watchlist, and everything needed to match a
-- release name against it without asking the API.
--
-- Matching is constrained rather than generic — the question is "does this
-- release belong to someone I follow", not "what is this release" — so the
-- alias tables are the working index and the entity tables are just identity.
-- Every alias carries a `normalised` column: `jane.doe`, `Jane Doe` and
-- `jane_doe` have to collide on lookup, and normalising at write time keeps
-- that off the hot path.
--
-- ids are stash-box UUIDs, kept as TEXT. `endpoint` is on every row because a
-- second stash-box (FansDB) is a different namespace for the same id space.

CREATE TABLE performers (
    endpoint       TEXT    NOT NULL,
    id             TEXT    NOT NULL,
    name           TEXT    NOT NULL,
    disambiguation TEXT    NOT NULL DEFAULT '',
    gender         TEXT    NOT NULL DEFAULT '',
    -- Followed by the account, as opposed to merely present: a synced scene
    -- drags in co-performers who are worth matching against but are not on
    -- the watchlist.
    is_favourite   INTEGER NOT NULL DEFAULT 0,
    deleted        INTEGER NOT NULL DEFAULT 0,
    -- stash-box merges duplicates; a merged performer keeps answering under
    -- its old id and points here. Matching follows the redirect.
    merged_into_id TEXT    NOT NULL DEFAULT '',
    updated_at     TEXT    NOT NULL DEFAULT '',
    synced_at      TEXT    NOT NULL,
    PRIMARY KEY (endpoint, id)
) STRICT;

CREATE INDEX performers_favourite ON performers (endpoint, is_favourite);

CREATE TABLE performer_aliases (
    endpoint     TEXT NOT NULL,
    performer_id TEXT NOT NULL,
    alias        TEXT NOT NULL,
    normalised   TEXT NOT NULL,
    PRIMARY KEY (endpoint, performer_id, alias),
    FOREIGN KEY (endpoint, performer_id) REFERENCES performers (endpoint, id) ON DELETE CASCADE
) STRICT;

-- The matcher's index: normalised alias -> performer.
CREATE INDEX performer_aliases_normalised ON performer_aliases (normalised);

CREATE TABLE performer_urls (
    endpoint     TEXT NOT NULL,
    performer_id TEXT NOT NULL,
    url          TEXT NOT NULL,
    site         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint, performer_id, url),
    FOREIGN KEY (endpoint, performer_id) REFERENCES performers (endpoint, id) ON DELETE CASCADE
) STRICT;

CREATE TABLE studios (
    endpoint     TEXT    NOT NULL,
    id           TEXT    NOT NULL,
    name         TEXT    NOT NULL,
    -- Network a site belongs to. Release names carry the site, so the parent
    -- is what connects one to the group that owns it. No foreign key: a
    -- parent is not necessarily favourited, so it may not be stored.
    parent_id    TEXT    NOT NULL DEFAULT '',
    is_favourite INTEGER NOT NULL DEFAULT 0,
    deleted      INTEGER NOT NULL DEFAULT 0,
    updated_at   TEXT    NOT NULL DEFAULT '',
    synced_at    TEXT    NOT NULL,
    PRIMARY KEY (endpoint, id)
) STRICT;

CREATE INDEX studios_favourite ON studios (endpoint, is_favourite);
CREATE INDEX studios_parent    ON studios (endpoint, parent_id);

CREATE TABLE studio_aliases (
    endpoint   TEXT NOT NULL,
    studio_id  TEXT NOT NULL,
    alias      TEXT NOT NULL,
    normalised TEXT NOT NULL,
    PRIMARY KEY (endpoint, studio_id, alias),
    FOREIGN KEY (endpoint, studio_id) REFERENCES studios (endpoint, id) ON DELETE CASCADE
) STRICT;

CREATE INDEX studio_aliases_normalised ON studio_aliases (normalised);

CREATE TABLE studio_urls (
    endpoint  TEXT NOT NULL,
    studio_id TEXT NOT NULL,
    url       TEXT NOT NULL,
    site      TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint, studio_id, url),
    FOREIGN KEY (endpoint, studio_id) REFERENCES studios (endpoint, id) ON DELETE CASCADE
) STRICT;

CREATE TABLE tags (
    endpoint TEXT NOT NULL,
    id       TEXT NOT NULL,
    name     TEXT NOT NULL,
    PRIMARY KEY (endpoint, id)
) STRICT;

CREATE TABLE scenes (
    endpoint     TEXT    NOT NULL,
    id           TEXT    NOT NULL,
    title        TEXT    NOT NULL DEFAULT '',
    -- The date a release name is most likely to carry, and the strongest
    -- signal after the studio.
    release_date TEXT    NOT NULL DEFAULT '',
    -- The studio's own scene code, where it publishes one.
    code         TEXT    NOT NULL DEFAULT '',
    duration     INTEGER NOT NULL DEFAULT 0,
    studio_id    TEXT    NOT NULL DEFAULT '',
    deleted      INTEGER NOT NULL DEFAULT 0,
    updated_at   TEXT    NOT NULL DEFAULT '',
    synced_at    TEXT    NOT NULL,
    PRIMARY KEY (endpoint, id)
) STRICT;

CREATE INDEX scenes_studio_date ON scenes (endpoint, studio_id, release_date);
CREATE INDEX scenes_date        ON scenes (endpoint, release_date);

CREATE TABLE scene_performers (
    endpoint     TEXT NOT NULL,
    scene_id     TEXT NOT NULL,
    performer_id TEXT NOT NULL,
    -- The alias this scene credited the performer under. Release names use
    -- the credited name far more often than the canonical one, which makes
    -- this the highest-value column in the schema for matching.
    as_alias     TEXT NOT NULL DEFAULT '',
    normalised   TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint, scene_id, performer_id),
    FOREIGN KEY (endpoint, scene_id) REFERENCES scenes (endpoint, id) ON DELETE CASCADE
) STRICT;

CREATE INDEX scene_performers_performer  ON scene_performers (endpoint, performer_id);
CREATE INDEX scene_performers_normalised ON scene_performers (normalised);

CREATE TABLE scene_tags (
    endpoint TEXT NOT NULL,
    scene_id TEXT NOT NULL,
    tag_id   TEXT NOT NULL,
    PRIMARY KEY (endpoint, scene_id, tag_id),
    FOREIGN KEY (endpoint, scene_id) REFERENCES scenes (endpoint, id) ON DELETE CASCADE
) STRICT;

CREATE TABLE scene_urls (
    endpoint TEXT NOT NULL,
    scene_id TEXT NOT NULL,
    url      TEXT NOT NULL,
    site     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint, scene_id, url),
    FOREIGN KEY (endpoint, scene_id) REFERENCES scenes (endpoint, id) ON DELETE CASCADE
) STRICT;

-- Fingerprints are how a downloaded file is confirmed against the scene it was
-- grabbed for. oshash is the one computable without decoding video.
CREATE TABLE scene_fingerprints (
    endpoint    TEXT    NOT NULL,
    scene_id    TEXT    NOT NULL,
    algorithm   TEXT    NOT NULL,
    hash        TEXT    NOT NULL,
    duration    INTEGER NOT NULL DEFAULT 0,
    submissions INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (endpoint, scene_id, algorithm, hash),
    FOREIGN KEY (endpoint, scene_id) REFERENCES scenes (endpoint, id) ON DELETE CASCADE
) STRICT;

-- Looking a file up by its hash is the whole point of storing these.
CREATE INDEX scene_fingerprints_hash ON scene_fingerprints (algorithm, hash);

-- One row per (endpoint, entity) recording how far the last crawl got, so a
-- refresh resumes instead of restarting and a failure is visible as a stale
-- timestamp rather than as silence.
CREATE TABLE sync_state (
    endpoint      TEXT    NOT NULL,
    entity        TEXT    NOT NULL,
    started_at    TEXT    NOT NULL DEFAULT '',
    finished_at   TEXT    NOT NULL DEFAULT '',
    cursor        TEXT    NOT NULL DEFAULT '',
    items         INTEGER NOT NULL DEFAULT 0,
    error         TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (endpoint, entity)
) STRICT;
