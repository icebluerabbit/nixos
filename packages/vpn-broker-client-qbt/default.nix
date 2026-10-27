/*
  vpn-broker-client-qbt — qBittorrent's client for the vpn-gw broker.

  Holds one tunnel claim, keeps it alive, and reconciles qbt's torrent
  session to the claimed tunnel's veth plus the forwarded port / announce IP
  the broker hands back. Two modes, `prestart` and `keepalive`, selected by
  argv[1]; argv[2] is the path to a JSON config.

  Shares its broker RPC and state-file handling with
  packages/vpn-broker-client-slskd — the two diverge only in how they steer
  the application (conf patch + WebUI API here, routing table there).
*/
{ writers }:
writers.writePython3Bin "vpn-broker-client-qbt" { } (builtins.readFile ./vpn_broker_client.py)
