# The Jellyfin library lost items. Not a Jellyfin fault — it is the sharpest available
# proxy for "the media mount went away underneath a container", and it catches the case
# where the NFS share disappears inside the container while the host still sees it.
#
# A negative delta of any size is abnormal: libraries grow, and deletions large enough
# to trip this are not something that happens by itself.
{ promRule, ... }:
let
  rule =
    {
      uid,
      title,
      metric,
      thing,
    }:
    promRule {
      inherit uid title;
      expr = "delta(${metric}[15m])";
      op = "lt";
      value = -5;
      lookback = 1800;
      for = "5m";
      severity = "critical";
      category = "jellyfin";
      noDataState = "OK";
      summary = "Jellyfin lost ${thing}";
      description = "Jellyfin's ${thing} count dropped in the last 15m. Libraries don't shrink on their own — check that the media share is still mounted inside the container.";
    };
in
{
  group = "media";
  rules = [
    (rule {
      uid = "jellyfin_movies_lost";
      title = "Jellyfin movies disappeared";
      metric = "jellyfin_library_movies";
      thing = "movies";
    })
    (rule {
      uid = "jellyfin_episodes_lost";
      title = "Jellyfin episodes disappeared";
      metric = "jellyfin_library_episodes";
      thing = "episodes";
    })
  ];
}
