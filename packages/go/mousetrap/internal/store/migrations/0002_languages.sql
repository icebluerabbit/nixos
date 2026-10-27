-- Languages are multi-valued (a release can be tagged with several). SQLite has no
-- array type, so normalize them into a child table instead of stuffing a delimited
-- string into announces.language. language_name (the single value parsed from the
-- release-name suffix, reliable across formats) stays a column on announces.
ALTER TABLE announces ADD COLUMN language_name TEXT;
ALTER TABLE announces DROP COLUMN language;

CREATE TABLE announce_languages (
  announce_id INTEGER NOT NULL REFERENCES announces(id) ON DELETE CASCADE,
  language    TEXT NOT NULL
);
CREATE INDEX idx_announce_languages_aid ON announce_languages(announce_id);
