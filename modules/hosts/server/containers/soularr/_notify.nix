# Vector notify producer for soularr → MQTT homelab/notify/soulseek → HA → #soulseek room.
# Journal-derived events (soularr has no webhook); each match publishes a small JSON payload
# that the HA _soulseek-webhook automation branches on by `.event`.
{ config }:
{
  extraSettings = {
    transforms.soularr_notify = {
      type = "remap";
      inputs = [ "journal" ];
      source = ''
        msg = to_string(.message) ?? ""
        ev = null
        album = null
        artist = null
        count = null
        album_id = null
        m = parse_regex(msg, r'Total Downloads added: (?P<n>\d+)') ?? null
        if !is_null(m) {
            if (to_int(m.n) ?? 0) > 0 {
                ev = "grabbed"
                count = m.n
            }
        }
        if is_null(ev) {
            m = parse_regex(msg, r'Completed download of Album: (?P<album>.+) Artist: (?P<artist>.+)') ?? null
            if !is_null(m) {
                ev = "downloaded"
                album = m.album
                artist = m.artist
            }
        }
        if is_null(ev) {
            # The trailing id is CAPTURED, not just matched. soularr prints a
            # Lidarr album id on this line and on no other, which makes this
            # the only one of its events that can be tied to a piece of
            # content — everything else it logs is album and artist as text,
            # and managarr resolves by identifier, never by title.
            m = parse_regex(msg, r'Added to failed import denylist: (?P<artist>.+) - (?P<album>.+) \(ID: (?P<album_id>\d+)\)') ?? null
            if !is_null(m) {
                ev = "import_failed"
                artist = m.artist
                album = m.album
                album_id = m.album_id
            }
        }
        if is_null(ev) {
            m = parse_regex(msg, r'Search failed for Album: (?P<album>.+) - Artist: (?P<artist>.+)') ?? null
            if !is_null(m) {
                ev = "search_failed"
                album = m.album
                artist = m.artist
            }
        }
        if is_null(ev) {
            abort
        }
        .message = encode_json({"source": "soularr", "event": ev, "album": album, "artist": artist, "count": count, "album_id": album_id})
      '';
    };
    sinks.soularr_notify_mqtt = {
      type = "mqtt";
      inputs = [ "soularr_notify" ];
      host = config.infra.clients.container-mosquitto.ip;
      port = config.infra.clients.container-mosquitto.ports.default;
      client_id = "vector-soularr"; # unique per container → no broker client-id collision
      topic = "homelab/notify/soulseek";
      quality_of_service = "atleastonce";
      retain = false;
      encoding.codec = "text"; # emit .message (the single-event JSON)
    };
    # Second consumer of the same transform — the acquisition half of the
    # Soulseek path. Lidarr's import webhook already covers the library half.
    sinks.soularr_notify_managarr = {
      type = "http";
      inputs = [ "soularr_notify" ];
      uri = "http://${config.infra.clients.container-managarr.ip}:${toString config.infra.clients.container-managarr.ports.default}/hook/soularr";
      method = "post";
      encoding.codec = "text";
      request.headers."Content-Type" = "application/json";
    };
  };
}
