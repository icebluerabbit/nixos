-- Aliases that name a network rather than the studio they are attached to.
--
-- Every label in a network lists the network's website as its own home, so the
-- derived domain alias lands on all of them: "familystrokes" is an alias of
-- fourteen studios — Dad Crush, PervMom, Sis Loves Me, Tiny Sis — and
-- "teamskeet" of forty more. None of them is identified by it.
--
-- 0007 tried to fix this by finding the studio whose name the alias really is,
-- which fails when the network is named "Family Strokes (Network)" and so
-- matches nothing. The general rule needs no such lookup: an alias shared by
-- more than a few studios cannot identify one, unless it is that studio's own
-- name, which is a fact about the studio rather than an inference from a URL.
DELETE FROM studio_aliases
 WHERE (endpoint, normalised) IN (
    SELECT endpoint, normalised FROM studio_aliases
     GROUP BY endpoint, normalised HAVING count(DISTINCT studio_id) > 3
 )
   AND alias <> (
    SELECT s.name FROM studios s
     WHERE s.endpoint = studio_aliases.endpoint AND s.id = studio_aliases.studio_id
 );
