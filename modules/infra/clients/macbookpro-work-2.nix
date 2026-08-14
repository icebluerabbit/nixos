# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "macbookpro-work-2";
  resourceName = "macbookpro_work_2";
  name = "MacBookPro (work, wifi)";
  mac = "ee:17:4a:7c:e1:18";
  ip = "10.10.70.42";
}
