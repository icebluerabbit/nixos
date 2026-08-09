/*
  audio — PipeWire audio subsystem; enables ALSA, PulseAudio compat, JACK, and rtkit real-time scheduling
  https://pipewire.org
*/
_: {
  den.aspects.hw._.audio = {
    aspect-name = "hw.audio";
    nixos = {
      security.rtkit.enable = true;
      services.pipewire = {
        enable = true;
        alsa.enable = true;
        alsa.support32Bit = true;
        pulse.enable = true;
        jack.enable = true;
      };
    };
  };
}
