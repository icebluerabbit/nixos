# home network
{ den, ... }:
let
  provider = (import ../_mk-provider.nix null) {
    secrets = {
      unifi-api-key = { };
      unifi-local-api-key = { };
      wifi-passphrase = { };
      radius-secret = { };
      dyndns-password = { };
    };
  };
in
{
  den.aspects.infra._.providers._.unifi = {
    aspect-name = "infra.providers.unifi";
    includes = [
      den.aspects.security._.sops
    ];

    provides = provider.mkProvides;

    terranix = {
      terraform.required_providers.unifi = {
        source = "ubiquiti-community/unifi";
        version = "0.49.0";
      };

      provider.unifi = {
        # Credentials are read from UNIFI_API_KEY environment variable at runtime
        api_url = "https://10.100.10.1";
        allow_insecure = true; # Set to true if using self-signed certs
      };

      variable.wifi_passphrase = {
        type = "string";
        sensitive = true;
      };
    };
  };
}
