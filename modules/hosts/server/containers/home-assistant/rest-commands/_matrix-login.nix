# Log ha-bot into tuwunel to mint a short-lived access token for CS-API reads
# (the grab flow). Password is passed as call data (data.password) so no secret
# is embedded here; reuses the existing matrix_ha_bot_password.
{ config, ... }:
{
  matrix_login = {
    url = "http://${config.infra.clients.container-tuwunel.ip}:${toString config.infra.clients.container-tuwunel.ports.default}/_matrix/client/v3/login";
    method = "POST";
    content_type = "application/json";
    payload = ''{"type":"m.login.password","identifier":{"type":"m.id.user","user":"ha-bot"},"password":"{{ password }}"}'';
  };
}
