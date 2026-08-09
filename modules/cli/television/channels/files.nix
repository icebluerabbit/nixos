{ den, ... }:
{
  den.aspects.cli._.television._.files = {
    aspect-name = "cli.television.files";
    includes = [
      den.aspects.cli._.bat
      den.aspects.cli._.eza
      den.aspects.cli._.fd
    ];

    homeManager =
      { pkgs, lib, ... }:
      let
        fd = lib.getExe pkgs.fd;
        eza = lib.getExe pkgs.eza;
        bat = lib.getExe pkgs.bat;
        tv = lib.getExe pkgs.television;
        file = lib.getExe pkgs.file;
        sort = lib.getExe' pkgs.coreutils "sort";
      in
      {
        programs.television.channels.files = {
          metadata = {
            name = "files";
            description = "A channel to select files and directories";
          };
          keybindings = {
            shortcut = "f1";
            f12 = "actions:edit";
            ctrl-up = "actions:goto_parent_dir";
          };
          preview = {
            command = ''
              if [ -d "{}" ]; then
                ${eza} -la --color=always --icons=always --group-directories-first "{}"
              else
                ${bat} -n --color=always --style=numbers,changes "{}" 2>/dev/null || ${file} -b "{}"
              fi
            '';
          };
          source = {
            command = "${fd} . --type f --type d --hidden --exclude .git --color=never | ${sort}";
          };
          actions = {
            edit = {
              description = "Opens the selected entries with the default editor (falls back to vim)";
              command = "\${EDITOR:-vim} '{}'";
              mode = "execute";
            };
            goto_parent_dir = {
              description = "Re-opens tv in the parent directory";
              command = "${tv} files ..";
              mode = "execute";
            };
          };
        };
      };
  };
}
