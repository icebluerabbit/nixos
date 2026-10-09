-- Languages are reduced to a single set at ingestion (autobrr's list, or the
-- name-parsed fallback when the list is empty). Only announce_languages is kept;
-- the separate language_name column is redundant.
ALTER TABLE announces DROP COLUMN language_name;
