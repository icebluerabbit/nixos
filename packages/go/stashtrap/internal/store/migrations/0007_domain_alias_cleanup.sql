-- Derived domain aliases that are really another studio's name.
--
-- A studio's aliases include the host of its own website, which is how
-- "AdultTime.com" in a release name finds the studio StashDB calls "Adult
-- Time". But a network's sub-studios list the network's domain as their home:
-- every TeamSkeet label — After Dark, Anal Euro, Bad MILFs, forty more — ends
-- up answering to "teamskeet", which is the name of the network itself.
--
-- So a derived alias is dropped where it is some other studio's actual name.
-- The network keeps it, the labels lose it, and a release naming the network
-- resolves to the network.
DELETE FROM studio_aliases
 WHERE (endpoint, studio_id, alias) IN (
    SELECT sa.endpoint, sa.studio_id, sa.alias
      FROM studio_aliases sa
      JOIN studios owner
        ON owner.endpoint = sa.endpoint
       AND owner.id <> sa.studio_id
      JOIN studio_aliases owner_alias
        ON owner_alias.endpoint = owner.endpoint
       AND owner_alias.studio_id = owner.id
       AND owner_alias.alias = owner.name
       AND owner_alias.normalised = sa.normalised
     WHERE sa.alias <> (SELECT name FROM studios s
                         WHERE s.endpoint = sa.endpoint AND s.id = sa.studio_id)
 );
