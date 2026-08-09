{ den, ... }:
{
  den.aspects.cli._.television._.dirs = {
    aspect-name = "cli.television.dirs";
    includes = [
      den.aspects.cli._.eza
      den.aspects.cli._.fd
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        fd = lib.getExe pkgs.fd;
        eza = lib.getExe pkgs.eza;
        tv = lib.getExe pkgs.television;
      in
      {
        programs.television.channels.dirs = {
          metadata = {
            name = "dirs";
            description = "A channel to select from directories";
          };
          keybindings = {
            shortcut = "f2";
          };
          preview = {
            command = "${eza} -l --color=always --icons=always --group-directories-first \"{}\"";
          };
          source = {
            command = "${fd} -t d";
          };
          actions = {
            cd = {
              description = "Open a shell in the selected directory";
              command = "cd '{}' && $SHELL";
              mode = "execute";
            };
            goto_parent_dir = {
              description = "Re-opens tv in the parent directory";
              command = "${tv} dirs ..";
              mode = "execute";
            };
          };
        };
      };
  };
}
