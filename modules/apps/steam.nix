/*
  Steam — Valve game client + Proton compatibility layer
  https://store.steampowered.com/
  System-scoped (programs.steam is a NixOS option, not home-manager) — pushed to
  host via provides.to-hosts.nixos. nvidia/Wayland env + AMD microcode come from
  hw.nvidia-rtx-4090 and hw.amd-9800-x3d; not duplicated here.
*/
_: {
  den.aspects.apps._.steam = {
    aspect-name = "apps.steam";
    includes = [ ];

    provides.to-hosts.nixos =
      { pkgs, ... }:
      {
        programs = {
          steam = {
            enable = true;
            # Translate X11 input events to uinput so Steam Input (controller
            # remapping, per-game bindings) works under Wayland/niri.
            extest.enable = true;
            # Winetricks wrapper for fixing individual Proton game prefixes.
            protontricks.enable = true;
            package = pkgs.steam.override {
              # pactl into Steam's FHS so its audio/mic controls see PipeWire
              # (pipewire-pulse); without it Steam logs `pactl: command not found`.
              extraPkgs = pkgs': [ pkgs'.pulseaudio ];
              # extraLibraries feeds multiPkgs, and Steam's FHS is multiArch, so extest
              # is built for both ABIs into /usr/lib and /usr/lib32. Those are ABI-split
              # default search dirs, so preloading by bare soname below resolves per
              # process. The nixpkgs module instead hardcodes the i686 path, which every
              # 64-bit Steam process rejects ("wrong ELF class: ELFCLASS32") — noise, and
              # it leaves 64-bit processes without extest at all. ld.so's $LIB/$PLATFORM
              # tokens do not help here: $LIB is `lib` for both ABIs on NixOS glibc.
              extraLibraries = pkgs': [ pkgs'.extest ];
              extraEnv = {
                # Steam's client UI ignores niri's fractional scaling; force it so the
                # window isn't tiny on 4K. Tune per display.
                STEAM_FORCE_DESKTOPUI_SCALING = "1.5";
                # Overrides the module's hardcoded i686 path (prev.extraEnv merges last).
                LD_PRELOAD = "libextest.so";
              };
            };
            # Adds a "Steam" gamescope session to the display manager (Big Picture / couch).
            # Off — host autologins straight into niri, nothing starts on its own.
            # Use `gamescope -- %command%` per-game instead.
            gamescopeSession.enable = false;
            extraCompatPackages = [ pkgs.proton-ge-bin ];
          };

          # Nested gamescope for demanding titles under niri: `gamescope -f -- %command%`.
          # Fixes fullscreen/cursor/VRR handling that native Wayland compositors mishandle.
          gamescope.enable = true;

          # On-demand performance: games that request GameMode get governor →
          # performance, GPU perf mode, and niceness bumps while running.
          gamemode.enable = true;
        };

        # udev rules for Steam Controller / DualSense / Xbox pads / Vive.
        hardware.steam-hardware.enable = true;
      };
  };
}
