# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "macbookpro-work-1";
  resourceName = "macbookpro_work_1";
  name = "MacBookPro (work 1)";
  mac = "a0:ce:c8:a0:f1:27";
  ip = "10.10.70.243";
}
