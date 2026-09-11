-- Strip the stray quotes already stored in category names.
--
-- Prowlarr ships a BitPorn definition whose every category description carries
-- an unbalanced double quote (`- {id: 20, cat: XXX, desc: Fansite"}`), so every
-- row written before types.cleanCategoryName existed carries it too. Names are
-- stored once rather than per announce, which is what makes this a dozen-row
-- update rather than a rewrite of the corpus.
--
-- Idempotent: trimming a name with no quotes leaves it unchanged, so a re-run
-- or a later definition fix costs nothing.
UPDATE categories
SET name = trim(trim(name), '"''')
WHERE name <> trim(trim(name), '"''');
