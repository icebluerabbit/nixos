# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "macbookpro-personal";
  resourceName = "macbookpro_personal";
  name = "MacBookPro (personal)";
  mac = "66:87:b9:92:9c:77";
  ip = "10.10.20.86";
}
