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
    # Instead make the headset the ONLY audio device: disable every ALSA card
    # that isn't the Arctis, plus every non-Arctis sink node as a backstop
    # (negated regex — no unrelated device IDs). With one sink left it is always
    # the default, and EasyEffects (useDefaultOutputDevice=true) follows it.
    #
    # Disabling at the card level takes that card's SOURCES with it, so the
    # Generic USB Audio mic and line-in are gone too and the Arctis mono mic is
    # the only capture device left — i.e. the headset is now unconditionally
    # default for input as well, which the sink-only rule never guaranteed.
    #
    # The card rule is what keeps this honest. Disabling only the node leaves
    # the card published with an active profile still advertising a sink that no
    # longer has a node behind it — EasyEffects walks the device list looking for
    # a node per card, finds none, and logs
    #   pw_manager.cpp:274 Could not find a node related to alsa_card.<name>
    # on every start. device.disabled makes wireplumber return before creating
    # the device at all (alsa.lua:548), so there is no card making the claim.
    nixos = {
      # snd_usb_audio normally autoloads on udev hotplug, which on a slow boot
      # can land within a second of wireplumber starting. ACP then probes the
      # still-initialising card, builds zero profiles, and the card sticks at
      # profile "off" — with every other sink disabled below, that is total
      # silence. Loading the module in stage 2 puts the card well ahead of the
      # user session.
      boot.kernelModules = [ "snd_usb_audio" ];

      services.pipewire.wireplumber.extraConfig."51-arctis-only-output" = {
        "monitor.alsa.rules" = [
          {
            matches = [
              { "device.name" = "!~alsa_card.usb-SteelSeries_Arctis_Nova_Pro_Wireless.*"; }
            ];
            actions.update-props."device.disabled" = true;
          }
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
