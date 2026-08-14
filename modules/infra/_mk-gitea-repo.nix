/*
  _mk-gitea-repo.nix — constructor for `infra.gitea.repos.*` aspects.

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  A repo created by migration keeps its migration settings under
  `ignore_changes`: the source address and token describe a one-time clone, and
  the provider reports them back empty afterwards, so tracking them would plan a
  re-migration on every run.
*/
{ den, lib }:
{
  id,
  resourceName,
  username,
  name,
  importId,
  description ? "",
  private ? false,
  mirror ? false,
  migrationCloneAddress ? null,
  migrationService ? null,
  migrationTokenVariable ? null,
}:
assert lib.assertMsg (
  (migrationCloneAddress == null) == (migrationService == null)
) "mkGiteaRepo ${id}: migrationCloneAddress and migrationService go together";
{
  den.aspects.infra._.gitea._.repos._.${id} = {
    aspect-name = "infra.gitea.repos.${id}";
    includes = [
      den.aspects.infra._.providers._.gitea
    ];

    terranix = {
      resource.gitea_repository.${resourceName} = {
        inherit
          username
          name
          description
          private
          mirror
          ;
      }
      // lib.optionalAttrs (migrationCloneAddress != null) {
        migration_clone_address = migrationCloneAddress;
        migration_service = migrationService;
        migration_service_auth_token = "\${var.${migrationTokenVariable}}";
        lifecycle = {
          ignore_changes = [
            "migration_clone_address"
            "migration_service"
            "migration_service_auth_token"
          ];
        };
      };

      import = [
        {
          to = "gitea_repository.${resourceName}";
          id = importId;
        }
      ];
    };
  };
}
