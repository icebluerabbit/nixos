-- Categories, normalised.
--
-- autobrr fills rls.Categories for a feed release (rls.Category stays empty —
-- that one is IRC-only), interleaving each category's name and numeric id. On
-- BitPorn that is always exactly four elements: the constant standard bucket
-- ["XXX","6000"] plus one site-specific pair such as ["Fansite","100020"].
--
-- A join table rather than a JSON column because JSON in TEXT is not
-- indexable: every "which announces are in category X" would be a full scan
-- through json_each(). It also means an upstream name defect is stored once
-- rather than on every row — Prowlarr's BitPorn caps currently give every
-- site-specific name a trailing double quote (`Fansite"`), and fixing that
-- upstream should update one row here, not twelve thousand.
CREATE TABLE categories (
    -- The tracker's own category id, used directly: 6000 is the standard
    -- Torznab bucket, 1000xx are BitPorn's own.
    id   INTEGER PRIMARY KEY,
    name TEXT    NOT NULL
) STRICT;

CREATE TABLE announce_categories (
    announce_id INTEGER NOT NULL REFERENCES announces(id) ON DELETE CASCADE,
    category_id INTEGER NOT NULL REFERENCES categories(id),
    PRIMARY KEY (announce_id, category_id)
) STRICT;

-- The reverse lookup: every announce in a category. The forward one is served
-- by the primary key.
CREATE INDEX announce_categories_category ON announce_categories (category_id);
