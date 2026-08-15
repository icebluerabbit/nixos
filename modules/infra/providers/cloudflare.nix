# Cloudflare provider configuration
{ den, ... }:
let
  provider = (import ../_mk-provider.nix null) {
    secrets = {
      cloudflare-api-token = { };
      cloudflare-zone-name = { };
      cloudflare-record-name = { };
      cloudflare-account-id = { };
      cloudflare-zone-id = { };
      wan-ip = { };
    };
  };
in
{
  den.aspects.infra._.providers._.cloudflare = {
    aspect-name = "infra.providers.cloudflare";
    includes = [
      den.aspects.security._.sops
    ];

    provides = provider.mkProvides;

    terranix = {
      terraform.required_providers.cloudflare = {
        source = "cloudflare/cloudflare";
        version = "5.19.1";
      };

      provider.cloudflare = {
        # Reads CLOUDFLARE_API_TOKEN environment variable by default
      };

      variable = {
        cf_zone_name = {
          type = "string";
          sensitive = true;
        };
        cf_record_name = {
          type = "string";
          sensitive = true;
        };
        cf_account_id = {
          type = "string";
          sensitive = true;
        };
        cf_zone_id = {
          type = "string";
        };
        wan_ip = {
          type = "string";
          sensitive = true;
        };
      };
    };
  };
}
