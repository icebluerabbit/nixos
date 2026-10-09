-- MAM changed the #announce format on 2026-08-11 so a release carries MULTIPLE
-- categories ("Fantasy, LGBTQIA+, Romance") plus a separate media type ("Ebook",
-- "Audiobook", "Periodical Ebook"). announces.category held the old single
-- "<MainCat> - <SubCat>" string and cannot represent either; it is DEPRECATED —
-- kept, never written again, so the pre-change history stays queryable verbatim.
ALTER TABLE announces ADD COLUMN media_type TEXT;

CREATE TABLE announce_categories (
  announce_id INTEGER NOT NULL REFERENCES announces(id) ON DELETE CASCADE,
  category    TEXT NOT NULL
);
CREATE INDEX idx_announce_categories_aid ON announce_categories(announce_id);

-- backfill from the deprecated column so metrics/queries span the format change.
-- Rows with no " - " (e.g. "Music Book") are a media type only, with no category.
UPDATE announces SET media_type =
  CASE WHEN INSTR(category, ' - ') > 0
       THEN SUBSTR(category, 1, INSTR(category, ' - ') - 1)
       ELSE category END
WHERE category IS NOT NULL;

INSERT INTO announce_categories (announce_id, category)
SELECT id, TRIM(SUBSTR(category, INSTR(category, ' - ') + 3))
FROM announces
WHERE category IS NOT NULL AND INSTR(category, ' - ') > 0;

-- The filetype is a list too ("azw3, epub"), and announces.filetype held it as a raw
-- string, so an exact allowlist match never fired on a multi-format release — 7 of
-- the first 1236 announces were rejected with filetype_allow while carrying an
-- allowed format. Same normalization; the column is DEPRECATED on the same terms.
CREATE TABLE announce_filetypes (
  announce_id INTEGER NOT NULL REFERENCES announces(id) ON DELETE CASCADE,
  filetype    TEXT NOT NULL
);
CREATE INDEX idx_announce_filetypes_aid ON announce_filetypes(announce_id);

-- SQLite has no split(), so the backfill is one statement per element. Every
-- historical multi-filetype value has exactly two parts, so two cover the table; a
-- hypothetical third format would be dropped HERE ONLY — new rows are split in Go.
INSERT INTO announce_filetypes (announce_id, filetype)
SELECT id, TRIM(CASE WHEN INSTR(filetype, ',') > 0
                     THEN SUBSTR(filetype, 1, INSTR(filetype, ',') - 1)
                     ELSE filetype END)
FROM announces WHERE filetype IS NOT NULL AND TRIM(filetype) <> '';

INSERT INTO announce_filetypes (announce_id, filetype)
SELECT id, TRIM(SUBSTR(filetype, INSTR(filetype, ',') + 1))
FROM announces WHERE INSTR(filetype, ',') > 0;
