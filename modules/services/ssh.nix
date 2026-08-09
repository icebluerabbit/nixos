/*
  OpenSSH — SSH daemon for remote access; public-key auth, agent forwarding
  https://www.openssh.com
*/
_: {
  den.aspects.services._.ssh = {
    aspect-name = "services.ssh";
    nixos = {
      services.openssh.enable = true;
    };
  };
}
