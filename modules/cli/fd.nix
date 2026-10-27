/*
  fd — simple, fast and user-friendly alternative to find
  https://github.com/sharkdp/fd
*/
_: {
  den.aspects.cli._.fd = {
    aspect-name = "cli.fd";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.fd ];

        # fd reads this on every search, on top of .gitignore. Build output,
        # dependency trees and tool caches only ever add noise to a result
        # list, so they are skipped globally rather than per-repo.
        xdg.configFile."fd/ignore".text = ''
          .git/
          node_modules/
          .venv/
          venv/
          __pycache__/
          *.pyc
          .DS_Store
          .cache/
          *.min.js
          *.min.css
          package-lock.json
          yarn.lock
          dist/
          build/
          target/
          *.o
          *.a
          *.so
          *.dylib
          .next/
          .nuxt/
          coverage/
          .pytest_cache/
          .mypy_cache/
          .ruff_cache/
        '';
      };
  };
}
