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
      # and they have nothing to escape. Only fields mousetrap actually reads are sent;
      # notably NOT .Cookie, which would put the live mam_id session in the journal (and
      # in Loki via the Vector sidecar) on every announce.
      #
      # The JSON keys below are mousetrap's and say what the data IS. The macros on the
      # right are autobrr's and do NOT — since MAM's 2026-08-11 announce change (autobrr
      # PR #2593, mirrored in definitions/myanonamouse.yaml) each carries something its
      # name does not describe. They are not renameable: Release.MapVars is a hardcoded
      # lookup over a fixed var vocabulary (category/tags/releaseTags/…), so a capture
      # group named mediaType or filetypes parses fine and then reaches no Release field
      # at all. Read this table, not the macro names:
      #   .Category    — the MEDIA TYPE ("Ebook" / "Audiobook" / "Periodical Ebook")
      #   .Tags        — the CATEGORY LIST. autobrr splits the var on "," and trims each
      #                  element, so MouseBot's ",  " arrives normalized to ", ".
      #   .ReleaseTags — the FILETYPE, itself a list on some releases ("m4b, mp3")
      #   .Origin      — "Fiction" / "Non-Fiction". The one var our fork renames rather
      #                  than merely repurposes (upstream calls the group fictionType,
      #                  which MapVars drops); see definitions/myanonamouse.yaml. MAM
      #                  category names span both sides — Crime, Historical and
      #                  Paranormal each appear as fiction AND non-fiction — so this is
      #                  the only signal that separates them.
      #   .Freeleech   — from MAM's trailing VIP|Normal token. On this announce format
      #                  VIP is the only freeleech signal MAM emits, so vip == freeleech.
      #   language     — autobrr's []string as a real JSON array (rls-derived; [] on m4b)
      #   languageName — pulled from MAM's "... [Language / filetype]" name suffix,
      #                  bypassing rls (bracket-classes avoid backslashes, backticks avoid
      #                  quote escaping); toRawJson-wrapped like any string field
      webhook_data = ''
        {
          "name": {{ toRawJson .TorrentName }},
          "title": {{ toRawJson .Title }},
          "mediaType": {{ toRawJson .Category }},
          "categories": {{ toRawJson .Tags }},
          "filetypes": {{ toRawJson .ReleaseTags }},
          "fiction": {{ toRawJson .Origin }},
          "language": {{ toRawJson .Language }},
          "languageName": {{ toRawJson (regexReplaceAll `^.*[[](.+?) / .+?[]]$` .TorrentName `$1`) }},
          "size": "{{ .Size }}",
          "vip": "{{ .Freeleech }}",
          "freeleech": "{{ .Freeleech }}",
          "freeleechPercent": "{{ .FreeleechPercent }}",
          "torrentId": "{{ .TorrentID }}",
          "downloadUrl": {{ toRawJson .DownloadUrl }}
        }
      '';
      webhook_headers = [ "Content-Type: application/json" ];
    }
  ];
}
