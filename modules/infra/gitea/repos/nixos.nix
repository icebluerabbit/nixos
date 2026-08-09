{ den, ... }:
{
  den.aspects.infra._.gitea._.repos._.nixos = {
    aspect-name = "infra.gitea.repos.nixos";
    includes = [
      den.aspects.infra._.providers._.gitea
    ];

    terranix = {
      resource.gitea_repository.nixos = {
        username = "admin";
        name = "nixos";
        description = "NixOS homelab configuration";
        private = false;
        migration_clone_address = "https://github.com/icebluerabbit/nixos.git";
        migration_service = "github";
        migration_service_auth_token = "\${var.github_token}";
        mirror = false;
        lifecycle = {
          ignore_changes = [
            "migration_clone_address"
            "migration_service"
            "migration_service_auth_token"
          ];
        };
      };
    };
  };
}
