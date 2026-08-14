# home network
{ den, ... }:
(import ../_mk-vpn.nix { inherit den; }) {
  id = "one-click-vpn";
  resourceName = "one_click_vpn";
  displayName = "One-Click VPN";
  unifiId = "6a2cdd81efa3052c88b365dc";
  subnet = "192.168.4.1/24";
}
