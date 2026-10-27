/*
  vpn-broker — the vpn-gw tunnel broker daemon.

  Owns tunnel selection, health, the provider port-forward lease and the
  mutable nftables dnat map for the download VLAN. Clients (see
  packages/vpn-broker-client-qbt and -slskd) claim a tunnel over its HTTP API
  and keep the claim alive.

  Config comes from $BROKER_CONFIG (a JSON file) and the binaries it shells
  out to from $BROKER_NFT / $BROKER_NATPMPC / $BROKER_IP, so the store paths
  are injected by the caller rather than resolved here.
*/
{ writers, python3Packages }:
writers.writePython3Bin "vpn-broker" {
  libraries = [ python3Packages.flask ];
} (builtins.readFile ./broker.py)
