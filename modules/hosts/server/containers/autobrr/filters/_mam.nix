# Catch-all MAM forwarder: match EVERY MAM announce and POST it to mousetrap.
# All policy (VIP exclusion, unsat-limit gate, ebook/audiobook routing, grab)
# lives in the sink — autobrr just parses + forwards. releaseTags + category
# ride the payload so mousetrap can decide.
{ config, ... }:
let
  mt = config.infra.clients.container-mousetrap;
in
{
  name = "MAM";
  enabled = true;
  indexers = [ { id = "\${ref.indexer.MyAnonamouse}"; } ];
  # MUST stay declared (even empty): an earlier version set except_release_tags="VIP" to
  # drop VIP announces. configuratarr syncs declared fields but does NOT prune ones you
  # stop declaring — remove this and the live filter keeps rejecting every VIP release.
  # Catch-all forwarder: mousetrap owns all VIP/FL policy, so exclude nothing here.
  match_release_tags = "";
  except_release_tags = "";
  actions = [
    {
      name = "mousetrap";
      action_type = "WEBHOOK";
      enabled = true;
      webhook_host = "http://${mt.ip}:${toString mt.ports.default}/announce";
      webhook_method = "POST";
      webhook_type = "JSON";
      # Hand-written JSON body (autobrr templates it, then POSTs). String fields use
      # {{ toRawJson .X }} (unquoted — toRawJson emits the quotes AND escapes), so a title
      # containing a " or \ can't produce an invalid body. Numeric/bool fields autobrr
      # types as int/uint64/bool stay quoted "{{ .X }}" — mousetrap wants them as strings
      # and they have nothing to escape.
      #   language     — autobrr's []string as a real JSON array (rls-derived; [] on m4b)
      #   languageName — pulled from MAM's "... [Language / filetype]" name suffix,
      #                  bypassing rls (bracket-classes avoid backslashes, backticks avoid
      #                  quote escaping); toRawJson-wrapped like any string field
      webhook_data = ''
        {
          "name": {{ toRawJson .TorrentName }},
          "title": {{ toRawJson .Title }},
          "category": {{ toRawJson .Category }},
          "categories": {{ toRawJson .Categories }},
          "uploader": {{ toRawJson .Uploader }},
          "tags": {{ toRawJson .Tags }},
          "language": {{ toRawJson .Language }},
          "languageName": {{ toRawJson (regexReplaceAll `^.*[[](.+?) / .+?[]]$` .TorrentName `$1`) }},
          "year": "{{ .Year }}",
          "type": {{ toRawJson .Type }},
          "origin": {{ toRawJson .Origin }},
          "size": "{{ .Size }}",
          "sizeString": {{ toRawJson .SizeString }},
          "releaseTags": {{ toRawJson .ReleaseTags }},
          "freeleech": "{{ .Freeleech }}",
          "freeleechPercent": "{{ .FreeleechPercent }}",
          "torrentId": "{{ .TorrentID }}",
          "torrentUrl": {{ toRawJson .TorrentUrl }},
          "downloadUrl": {{ toRawJson .DownloadUrl }},
          "infoUrl": {{ toRawJson .InfoUrl }},
          "magnetUri": {{ toRawJson .MagnetURI }},
          "cookie": {{ toRawJson .Cookie }},
          "protocol": {{ toRawJson .Protocol }},
          "implementation": {{ toRawJson .Implementation }},
          "indexer": {{ toRawJson .IndexerName }},
          "indexerIdentifier": {{ toRawJson .IndexerIdentifier }}
        }
      '';
      webhook_headers = [ "Content-Type: application/json" ];
    }
  ];
}
