-- 0004 backfilled media_type verbatim from the deprecated category column, but MAM's
-- pre-2026-08-11 vocabulary was PLURAL ("Ebooks", "Audiobooks") while the new announce
-- emits SINGULAR ("Ebook", "Audiobook"). That split every by-media-type series in two
-- across the cutover — 780 "Ebooks" + 42 "Ebook", 442 "Audiobooks" + 14 "Audiobook".
-- Normalize the backfilled rows onto the current vocabulary; new rows already match.
UPDATE announces SET media_type = 'Ebook' WHERE media_type = 'Ebooks';
UPDATE announces SET media_type = 'Audiobook' WHERE media_type = 'Audiobooks';

-- "Radio" and "Music Book" (3 rows each, pre-cutover only) are left alone: no
-- post-cutover announce has carried either, so their current spelling is unknown.
