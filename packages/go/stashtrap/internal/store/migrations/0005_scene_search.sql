-- Scene titles, indexed for search rather than equality.
--
-- Exact title matching answers only when a release reproduces a title
-- character for character, which these rarely do: words are reordered, the
-- performer's name is prepended, punctuation differs, and one word in ten is
-- misspelled. A trigram index answers the question that was actually being
-- asked — which known scene does this text most look like — and bm25() gives a
-- number to rank the answers by.
--
-- Trigram rather than the default tokenizer: it matches inside words, so a
-- release that writes "Anthology" where the scene says "Anthology:" still
-- lines up, and a query OR-ing its tokens survives one of them being wrong.
CREATE VIRTUAL TABLE IF NOT EXISTS scene_search USING fts5 (
    title,
    scene_id UNINDEXED,
    endpoint UNINDEXED,
    tokenize = 'trigram'
);

-- Backfill what is already stored. Unlike the normalised-title column added in
-- 0004, this one can be filled in SQL, so the index is complete the moment the
-- migration runs rather than after the next crawl rewrites every scene.
INSERT INTO scene_search (title, scene_id, endpoint)
SELECT title, id, endpoint FROM scenes WHERE title <> '' AND deleted = 0;
