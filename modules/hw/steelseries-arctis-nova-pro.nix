/*
  steelseries-arctis-nova-pro — SteelSeries Arctis Nova Pro wireless headset configuration and integration with HeadsetControl
  https://github.com/SirenOfTiles/HeadsetControl
*/
{ den, ... }:
{
  den.aspects.hw._.steelseries-arctis-nova-pro = {
    aspect-name = "hw.steelseries-arctis-nova-pro";
    includes = [
      den.aspects.hw._.audio
      den.aspects.hw._.headset
    ];

    # EasyEffects 8.x moved off GSettings, so the old dconf sink pin is dead.
    # Instead make the headset the ONLY output: disable every ALSA sink node
    # that isn't the Arctis (negated regex — no unrelated device IDs). Sources
    # (mics) are untouched. With one sink left it is always the default, and
    # EasyEffects (useDefaultOutputDevice=true) follows it.
    nixos = {
      services.pipewire.wireplumber.extraConfig."51-arctis-only-output" = {
        "monitor.alsa.rules" = [
          {
            matches = [
              {
                "media.class" = "Audio/Sink";
                "node.name" = "!~alsa_output.usb-SteelSeries_Arctis_Nova_Pro_Wireless.*";
              }
            ];
            actions.update-props."node.disabled" = true;
          }
        ];
      };
    };
  };
}
