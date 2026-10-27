# Shared filter action: on every grab, POST the release to managarr so the
# decision is recorded at the point it is made.
#
# This is the head of the chain. Prowlarr also reports proxied grabs, but it
# never sees a downloadId — it hands the .torrent back and the *arr talks to
# the client — so its leg can only ever be matched on release title. autobrr
# knows which filter matched and which indexer it came from, which is the part
# that is otherwise unrecoverable once the release is in qBittorrent under a
# name that may have been rewritten.
#
# The hash macro is .TorrentHash, not .InfoHash — the latter does not exist in
# autobrr's domain.Macro and a template referencing it fails to parse, which
# fails the whole action rather than leaving the field empty. Every push to
# managarr errored this way until it was corrected.
#
# .TorrentHash is the reason this action exists at all: autobrr's own
# /api/release records carry no infohash, so this webhook is the only place
# autobrr ever emits the key that links an announce to a download.
#
# Same constraint as _cross-seed-action.nix applies: configuratarr does not
# sync webhook_headers, so anything that must reach managarr goes in the body
# or the URL.
{ config, ... }:
let
  m = config.infra.clients.container-managarr;
in
{
  name = "managarr";
  action_type = "WEBHOOK";
  enabled = true;
  webhook_host = "http://${m.ip}:${toString m.ports.default}/hook/autobrr";
  webhook_method = "POST";
  webhook_type = "JSON";
  webhook_data = builtins.toJSON {
    name = "{{ .TorrentName }}";
    infohash = "{{ .TorrentHash }}";
    indexer = "{{ .IndexerName }}";
    filter = "{{ .FilterName }}";
    size = "{{ .Size }}";
    category = "{{ .Category }}";
    url = "{{ .TorrentUrl }}";
    # The tracker's own torrent id, and the reason anything here can be
    # attached to the right announce. autobrr keys a release by a numeric id no
    # macro can emit, and emits an infohash its API never serialises, so
    # neither side alone can name an announce to the other. .TorrentID is the
    # one field present in both — managarr matches on
    # (indexer, tracker_torrent_id).
    torrent_id = "{{ .TorrentID }}";
    # Swarm counts at the moment of the announce. Both are `json:"-"` in
    # autobrr's domain model, so the API never reports them at any age: this
    # webhook is the only way they are ever recorded, and they are what the
    # decision to take or refuse the release was made against.
    seeders = "{{ .Seeders }}";
    leechers = "{{ .Leechers }}";
  };
  webhook_headers = [
    "Content-Type: application/json"
  ];
}
