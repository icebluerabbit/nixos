# vm — QEMU virtual machine runner configurations for local host testing
{ inputs, ... }:
{
  perSystem =
    { pkgs, ... }:
    {
      packages.vm-desktop = pkgs.writeShellApplication {
        name = "vm-desktop";
        text =
          let
            host = inputs.self.nixosConfigurations.desktop.config;
          in
          ''
            ${host.system.build.vm}/bin/run-${host.networking.hostName}-vm "$@"
          '';
      };

      packages.vm-server = pkgs.writeShellApplication {
        name = "vm-server";
        text =
          let
            host = inputs.self.nixosConfigurations.server.config;
          in
          ''
            mkdir -p /tmp/vm-server-mnt

            export QEMU_NET_OPTS="hostfwd=tcp::22222-:22"

            ${host.system.build.vm}/bin/run-${host.networking.hostName}-vm "$@"
          '';
      };
    };
}
