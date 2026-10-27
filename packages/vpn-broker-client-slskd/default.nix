/*
  vpn-broker-client-slskd — slskd's client for the vpn-gw broker.

  slskd/.NET has no per-app interface binding for outbound peer connections,
  so unlike the qbittorrent client this one steers by the ROUTING TABLE:
  default via the claimed tunnel veth, LAN carved out to eth0, blackhole when
  there is no claim. Runs as root (needs `ip route`). Two modes, `prestart`
  and `keepalive`, selected by argv[1]; argv[2] is the path to a JSON config.
*/
{ writers }:
writers.writePython3Bin "vpn-broker-client-slskd" { } (builtins.readFile ./slskd_broker_client.py)
