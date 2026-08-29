/*
  LM Studio — desktop app for running local LLMs (llama.cpp/MLX runtimes).
  https://lmstudio.ai/

  Unfree AppImage repack; nixpkgs wraps it with appimageTools, so the
  llama.cpp/CUDA runtimes it downloads at runtime resolve against that FHS
  env rather than needing nix-ld. Models and runtimes land in ~/.lmstudio,
  which sits on the persistent /home subvolume — nothing to add to
  hosts.desktop.impermanence.
*/
_: {
  den.aspects.apps._.lm-studio = {
    aspect-name = "apps.lm-studio";
    homeManager =
      { pkgs, lib, ... }:
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        home.packages = [ pkgs.lmstudio ];
      };
  };
}
