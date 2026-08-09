# Fetch a single room event by id via the Matrix Client-Server API — used to read
# a 👍'd reject card's hidden [Ref: …] marker. token/room/event_id passed as call data.
{ config, ... }:
{
  matrix_get_event = {
    url = "http://${config.infra.clients.container-tuwunel.ip}:${toString config.infra.clients.container-tuwunel.ports.default}/_matrix/client/v3/rooms/{{ room }}/event/{{ event_id }}";
    method = "GET";
    headers.Authorization = "Bearer {{ token }}";
  };
}
