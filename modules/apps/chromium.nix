/*
  chromium — Chromium browser with extensions, hardware acceleration, and privacy optimizations
  https://www.chromium.org/

  Target: Chromium 151+ on niri / Wayland / NVIDIA (RTX 4090, nvidia-vaapi-driver).

  Every feature and switch below was verified to exist in the Chromium 151 binary.
  Chromium silently ignores unknown --enable-features/--disable-features names, so a
  stale flag is invisible: it neither warns nor takes effect. The previous revision of
  this file carried 33 names that no longer exist (including the entire VA-API decode
  set), which is why hardware video decode was dead.

  Verify after a Chromium major bump:
    strings <chromium-unwrapped>/libexec/chromium/chromium | grep -x '<FeatureName>'
*/
_: {
  den.aspects.apps._.chromium = {
    aspect-name = "apps.chromium";
    includes = [ ];

    homeManager =
      { lib, pkgs, ... }:
      let
        features = {
          # --- Hardware video decode/encode (NVIDIA via nvidia-vaapi-driver) ---
          # This is the GL-backed VA-API path. It is the ONLY working decode path on
          # NVIDIA + Wayland: the Vulkan variant leaves the decoder unable to negotiate
          # an output format, so PickDecoderOutputFormat() fails and every HEVC stream
          # dies with PIPELINE_ERROR_DECODE before a frame is decoded. H.264 survived
          # that only because it negotiates NV12; HEVC has no software fallback in
          # Chromium, so it failed outright.
          AcceleratedVideoDecodeLinuxGL = true;
          AcceleratedVideoDecodeLinuxZeroCopyGL = true;
          AcceleratedVideoEncoder = true;
          # NVIDIA VA-API is off by default in Chromium and its capability reporting is
          # not trusted; both are required for nvidia-vaapi-driver to be used at all.
          VaapiOnNvidiaGPUs = true;
          VaapiIgnoreDriverChecks = true;
          # nvidia-vaapi-driver is not thread-safe across contexts.
          GlobalVaapiLock = true;
          VaapiImageProcessor = true;

          # --- Rendering backend ---
          # Vulkan is explicitly incompatible with the Wayland Ozone platform; Chromium
          # logs "'--ozone-platform=wayland' is not compatible with Vulkan" and DRM PRIME
          # buffer sharing goes unavailable, corrupting decoded video output. ANGLE runs
          # on GL instead (see --use-angle=gl below).
          Vulkan = false;
          DefaultANGLEVulkan = false;
          VulkanFromANGLE = false;
          # Chromium's wp-color-manager-v1 support washes out colours on this stack.
          WaylandWpColorManagerV1 = false;

          # --- Wayland integration ---
          WaylandTextInputV3 = true;
          WebRtcPipeWireCamera = true;

          # --- Performance ---
          V8Maglev = true;
          BackForwardCache = true;
          ParallelDownloading = true;

          # --- Privacy ---
          PrivacySandboxSettings4 = false;
          BrowsingTopics = false;
          BrowsingTopicsDocumentAPI = false;
          BrowsingTopicsParameters = false;
          FledgeConsiderKAnonymity = false;
        };

        enabledFeatures = builtins.attrNames (lib.attrsets.filterAttrs (_: v: v == true) features);
        disabledFeatures = builtins.attrNames (lib.attrsets.filterAttrs (_: v: v == false) features);
      in
      {
        programs.chromium = {
          enable = true;
          package = pkgs.chromium.override {
            enableWideVine = true;
          };
          extensions = [
            "ddkjiahejlhfcafbddmgiahcphecmpfh" # uBlock Origin Lite
            "gcbommkclmclpchllfjekcdonpmejbdp" # https-everywhere
            "pfkkfbfdhomeagojoahjmkojeeepcolc" # watchmarker-for-youtube
            "gebbhagfogifgggkldgodflihgfeippi" # return-youtube-dislike
            "ponfpcnoihfmfllpaingbgckeeldkhle" # enhancer-for-youtube
            "mnjggcdmjocbbbhaepdhchncahnbgone" # sponsorblock-for-youtube
            "eimadpbcbfnmbkopoojfekhnkhdbieeh" # dark-reader
            "hlepfoohegkhhmjieoechaddaejaokhf" # refined github
          ];
          commandLineArgs = [
            # Native Wayland. --ozone-platform-hint no longer exists in Chromium 151;
            # selecting the platform explicitly avoids depending on NIXOS_OZONE_WL.
            "--ozone-platform=wayland"
            "--gtk-version=4"

            # ANGLE on GL — see the Vulkan comment in `features` above.
            "--use-gl=angle"
            "--use-angle=gl"
            # Stable decoding path for Chromium 147+ on NVIDIA.
            "--use-cmd-decoder=passthrough"

            # GPU rasterization + zero-copy raster buffers.
            "--ignore-gpu-blocklist"
            "--enable-gpu-rasterization"
            "--enable-zero-copy"
            "--num-raster-threads=12"

            "--js-flags=\"--max-old-space-size=8192\""
            "--disk-cache-dir=/run/user/1000/chromium-cache"

            # Strict extension verification
            "--extension-content-verification=enforce_strict"
            "--extensions-install-verification=enforce_strict"

            # NOTE: --disable-reading-from-canvas (anti-canvas-fingerprinting) makes
            # Chromium throw "tainted canvas" SecurityError on ALL canvas readback
            # (getImageData/texImage2D-from-canvas), which breaks WebGL apps like the
            # code-server / VS Code web terminal (xterm reads back its glyph atlas).
            # Removed — re-enable only if you accept losing browser canvas/WebGL apps.
            "--no-first-run"
            "--no-default-browser-check"
            "--disable-speech-api"
            "--disable-speech-synthesis-api"
            "--disable-breakpad"
            "--no-pings"
            "--component-updater=require_encryption"
            "--disable-sync"
            "--restore-last-session"

            "--enable-features=${builtins.concatStringsSep "," enabledFeatures}"
            "--disable-features=${builtins.concatStringsSep "," disabledFeatures}"
          ];
        };
      };
  };
}
