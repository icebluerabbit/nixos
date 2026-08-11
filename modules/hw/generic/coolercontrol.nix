/*
  CoolerControl — fan and pump control daemon; Nix-generated TOML config, GUI via coolercontrol-liqctld
  https://github.com/codifryed/coolercontrol
*/
{ den, ... }:
{
  den.aspects.hw._.coolercontrol = {
    aspect-name = "hw.coolercontrol";
    includes = [
      den.aspects.hw._.sensor
      den.aspects.hw._.liquidctl
    ];
    nixos =
      {
        lib,
        pkgs,
        config,
        ...
      }:
      let
        cc = config.hardware.coolercontrol;

        baseSettings = {
          apply_on_boot = true;
          liquidctl_integration = true;
          hide_duplicate_devices = true;
          no_init = false;
          startup_delay = 2;
          compress_payload = true;
          drivetemp_suspend = false;
        };

        # Serializes a Nix value as a TOML inline value.
        # Attrsets always become inline tables { k = v, ... }.
        toVal =
          v:
          if builtins.isString v then
            ''"${v}"''
          else if builtins.isBool v then
            (if v then "true" else "false")
          else if builtins.isInt v then
            toString v
          else if builtins.isFloat v then
            let
              s = toString v;
            in
            if lib.hasInfix "." s then s else s + ".0"
          else if builtins.isList v then
            "[ " + lib.concatMapStringsSep ", " toVal v + " ]"
          else if builtins.isAttrs v then
            "{ " + lib.concatStringsSep ", " (lib.mapAttrsToList (k: w: "${k} = ${toVal w}") v) + " }"
          else
            throw "coolercontrol: unsupported value type for ${builtins.typeOf v}";

        mkProfile =
          p:
          let
            req = k: "${k} = ${toVal p.${k}}\n";
            opt = k: lib.optionalString (p ? ${k}) (req k);
          in
          "[[profiles]]\n"
          + req "uid"
          + req "name"
          + req "p_type"
          + opt "speed_fixed"
          + opt "speed_profile"
          + opt "temp_source"
          + opt "temp_min"
          + opt "temp_max"
          + opt "function_uid"
          + opt "function"
          + opt "offset_profile"
          + "\n";

        mkFunction =
          f:
          let
            req = k: "${k} = ${toVal f.${k}}\n";
            opt = k: lib.optionalString (f ? ${k}) (req k);
          in
          "[[functions]]\n"
          + req "uid"
          + req "name"
          + req "f_type"
          + opt "duty_minimum"
          + opt "duty_maximum"
          + opt "step_size_min_decreasing"
          + opt "step_size_max_decreasing"
          + opt "sample_window"
          + opt "threshold_hopping"
          + "\n";

        mkDeviceSettings =
          uid: channels:
          "[device-settings.${uid}]\n"
          + lib.concatStringsSep "\n" (lib.mapAttrsToList (ch: v: "${ch} = ${toVal v}") channels)
          + "\n";

        mkSettings =
          s:
          "[settings]\n"
          + lib.concatStringsSep "\n" (lib.mapAttrsToList (k: v: "${k} = ${toVal v}") s)
          + "\n";

        configStr = ''
          [devices]

          [legacy690]

          [device-settings]
          ${lib.concatStringsSep "\n" (lib.mapAttrsToList mkDeviceSettings cc.deviceSettings)}
          ${lib.concatMapStrings mkProfile cc.profiles}
          ${lib.concatMapStrings mkFunction cc.functions}
          ${mkSettings (baseSettings // cc.settings)}
        '';

        configFile = pkgs.writeText "coolercontrol-config.toml" configStr;
      in
      {
        options.hardware.coolercontrol = {
          profiles = lib.mkOption {
            type = lib.types.listOf (lib.types.attrsOf lib.types.anything);
            default = [
              {
                uid = "0";
                name = "Default Profile";
                p_type = "Default";
                function = "0";
              }
            ];
          };
          functions = lib.mkOption {
            type = lib.types.listOf (lib.types.attrsOf lib.types.anything);
            default = [
              {
                uid = "0";
                name = "Default Function";
                f_type = "Identity";
              }
            ];
          };
          deviceSettings = lib.mkOption {
            type = lib.types.attrsOf (lib.types.attrsOf lib.types.anything);
            default = { };
          };
          settings = lib.mkOption {
            type = lib.types.attrsOf lib.types.anything;
            default = { };
          };
        };

        config = {
          systemd.packages = [ pkgs.coolercontrol.coolercontrold ];
          systemd.services.coolercontrold = {
            wantedBy = [ "multi-user.target" ];
            # "+" runs it with full privileges, before the service's own user drop.
            serviceConfig.ExecStartPre = [
              "+${
                lib.escapeShellArgs [
                  (lib.getExe pkgs.homelab.coolercontrol-init-config)
                  "--config"
                  configFile
                  "--dest"
                  "/etc/coolercontrol/config.toml"
                ]
              }"
            ];
          };
        };
      };
  };
}
