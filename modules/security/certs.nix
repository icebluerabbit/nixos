# certs — install homelab CA certificate into the system trust store
{ inputs, ... }:
{
  den.aspects.security._.certs = {
    aspect-name = "security.certs";
    includes = [ ];

    nixos = _: {
      security.pki.certificateFiles = [ (inputs.self + "/certs/ca.crt") ];
    };
  };
}
