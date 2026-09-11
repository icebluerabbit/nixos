-- A release often names the scene rather than the people in it: "[Site] My
-- boyfriend wanted something new. I suggested him to have sex with my friend"
-- credits nobody, and no amount of parsing will find a performer in it. The
-- catalogue already knows that scene and who is in it, so the title itself is
-- a way in.
--
-- Populated as scenes are upserted; the daily crawl rewrites every row, so the
-- column fills itself within one sync rather than needing a backfill that
-- SQLite cannot express anyway.
ALTER TABLE scenes ADD COLUMN title_normalised TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_scenes_title_normalised
    ON scenes (endpoint, title_normalised);
