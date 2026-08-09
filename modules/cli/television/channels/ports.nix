{ den, ... }:
{
  den.aspects.cli._.television._.ports = {
    aspect-name = "cli.television.ports";
    includes = [
      den.aspects.cli._.psmisc
    ];

    homeManager =
      { pkgs, lib, ... }:
      # Relies on `ss` (iproute2) and `fuser` (psmisc), both Linux-only.
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux (
        let
          ss = lib.getExe' pkgs.iproute2 "ss";
          tail = lib.getExe' pkgs.coreutils "tail";
          head = lib.getExe' pkgs.coreutils "head";
          awk = lib.getExe pkgs.gawk;
          sed = lib.getExe' pkgs.gnused "sed";
          grep = lib.getExe pkgs.gnugrep;
          fuser = lib.getExe' pkgs.psmisc "fuser";
        in
        {
          programs.television.channels.ports = {
            metadata = {
              name = "ports";
              description = "List listening ports and associated processes";
            };
            source = {
              command = "${ss} -tlnp 2>/dev/null | ${tail} -n +2 | ${awk} '{gsub(/.*:/,\"\",$4); print $4, $1, $6}' | ${sed} 's/users:((\"//; s/\".*//'";
              display = "{split: :0} ({split: :2})";
            };
            preview = {
              command = "${ss} -tlnp 2>/dev/null | ${grep} ':{split: :0} ' | ${head} -20";
            };
            actions = {
              kill = {
                description = "Kill the process listening on the selected port";
                command = "${fuser} -k {split: :0}/tcp";
                mode = "execute";
              };
            };
          };
        }
      );
  };
}
