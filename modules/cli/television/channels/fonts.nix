_: {
  den.aspects.cli._.television._.fonts = {
    aspect-name = "cli.television.fonts";
    includes = [ ];

    homeManager =
      { pkgs, lib, ... }:
      let
        fc-list = lib.getExe' pkgs.fontconfig "fc-list";
        sort = lib.getExe' pkgs.coreutils "sort";
        head = lib.getExe' pkgs.coreutils "head";
      in
      {
        programs.television.channels.fonts = {
          metadata = {
            name = "fonts";
            description = "List installed system fonts";
          };
          source = {
            command = "${fc-list} --format='%{family}\\n' | ${sort} -uf";
          };
          preview = {
            command = "${fc-list} '{}' --format='%{file}\\n%{style}\\n%{family}\\n' | ${head} -20";
          };
        };
      };
  };
}
