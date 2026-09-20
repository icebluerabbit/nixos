# BitPorn — gated by stashtrap, which answers 250 for a release worth taking
# and anything else for one that is not. autobrr does the grab itself with the
# qBittorrent action below, so a stashtrap outage stops acquisition rather than
# corrupting it.
#
# Enabling this filter is safe while the matcher is still being built:
# stashtrap records every announce and currently accepts none.
#
# Releases arrive from a Torznab feed rather than IRC (BitPorn runs no announce
# network) — see ../feeds/_bitporn.nix.
#
# No freeleech gate here on purpose: BitPorn's Cardigann definition reports only
# PER-TORRENT freeleech, so during a site-wide freeleech event 87 of 100
# releases still came through as downloadvolumefactor=1. Gating on it would drop
# almost everything while everything was in fact free.
{ config, ... }:
let
  st = config.infra.clients.container-stashtrap;
in
{
  name = "BitPorn";
  enabled = true;
  indexers = [ { id = "\${ref.indexer.BitPorn}"; } ];
  # Match gate: stashtrap decides, and nothing is added to the client unless it
  # answers 250. Same shape as the seasonpackarr filter's external check.
  external = [
    {
      name = "stashtrap";
      external_type = "WEBHOOK";
      enabled = true;
      webhook_host = "http://${st.ip}:${toString st.ports.default}/announce";
      webhook_method = "POST";
      webhook_expect_status = 250;
      # Every field is passed through verbatim — toRawJson for anything not
      # already a JSON scalar, and nothing else. Normalising here is a trap: a
      # template has no tests, aborts at its first bad field so one mistake
      # hides the next, and turns a source defect into a stashtrap defect.
      # Reshaping belongs in internal/types, where it can be asserted.
      #
      # toRawJson is unquoted on purpose: it emits the quotes and escapes the
      # contents, so a release title carrying a quote or backslash cannot
      # produce an invalid body.
      #
      # Two fields still need care, and a bad one hides the next:
      #
      #   .TorrentHash — autobrr's macro context is domain.Macro, which has no
      #                  InfoHash field at all. Using .InfoHash renders nothing
      #                  and autobrr drops the release before stashtrap sees it.
      #   toString     — b64enc takes a string and TorrentDataRawBytes is
      #                  []byte, so the pipe is required, not decoration.
      #                  sprig's strval has an explicit []byte case, making the
      #                  conversion byte-exact; without it the %v default would
      #                  render the bencode as a decimal byte list. That field
      #                  is the .torrent: base64 because bencode is binary and
      #                  JSON strings must be valid UTF-8, so `js` escaping
      #                  would lose bytes to U+FFFD on decode.
      #
      # Field quirks worth knowing, all handled on the Go side:
      #
      #   .Categories  — plural, and NOT .Category: internal/feed/torznab.go
      #                  only fills rls.Categories on a feed release, as
      #                  [name, id] pairs flattened in order. Prowlarr's
      #                  BitPorn caps misspell every site-specific name with a
      #                  trailing double quote (`Fansite"`); it is recorded as
      #                  sent, and lives in one row of the categories table so
      #                  an upstream fix repairs it centrally.
      #   .Tags        — a STRING here, not a list: macros.go joins
      #                  release.Tags with ", ". types.parseTags already
      #                  accepts either form. Always "" on BitPorn, whose
      #                  torznab:attr genre is empty on every release.
      #   uploader,    — empty for a Torznab feed. The first two are absent
      #   torrent_id,    from BitPorn's feed entirely; info_url is dropped by
      #   info_url       autobrr 1.82.1, which only learned to fill rls.InfoURL
      #                  from <comments> on develop. Recorded empty on purpose:
      #                  phase one is for learning what actually arrives.
      webhook_data = ''
        {
          "name": {{ toRawJson .TorrentName }},
          "indexer": {{ toRawJson .Indexer }},
          "info_hash": {{ toRawJson .TorrentHash }},
          "torrent_id": {{ toRawJson .TorrentID }},
          "category": {{ toRawJson .Category }},
          "categories": {{ toRawJson .Categories }},
          "tags": {{ toRawJson .Tags }},
          "release_tags": {{ toRawJson .ReleaseTags }},
          "size": {{ .Size }},
          "freeleech": {{ .Freeleech }},
          "freeleech_percent": {{ .FreeleechPercent }},
          "uploader": {{ toRawJson .Uploader }},
          "info_url": {{ toRawJson .InfoUrl }},
          "torrent_data": "{{ .TorrentDataRawBytes | toString | b64enc }}"
        }
      '';
      # A string here, not a list: an external check's headers are
      # `Key=value,Key2=value2`, unlike an action's, which take a list.
      webhook_headers = "Content-Type=application/json";
    }
  ];
  actions = [
    {
      name = "qBittorrent";
      action_type = "QBITTORRENT";
      enabled = true;
      client_id = "\${ref.download_client.qBittorrent}";
      category = "xxx";
    }
    (import ./_cross-seed-action.nix { inherit config; })
    (import ./_managarr-action.nix { inherit config; })
  ];
}
