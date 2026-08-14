# home network
{ den, lib, ... }:
(import ../_mk-client.nix { inherit den lib; }) {
  id = "google-home";
  resourceName = "google_home";
  name = "Google Home";
  mac = "f4:f5:d8:b4:ff:78";
  ip = "10.10.30.188";
}
