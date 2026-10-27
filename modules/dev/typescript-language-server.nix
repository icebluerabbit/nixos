/*
  typescript-language-server — LSP server for JavaScript and TypeScript, wrapping tsserver
  https://github.com/typescript-language-server/typescript-language-server
*/
_: {
  den.aspects.dev._.typescript-language-server = {
    aspect-name = "dev.typescript-language-server";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.typescript-language-server ];
      };
  };
}
