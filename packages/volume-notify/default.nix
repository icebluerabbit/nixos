/*
  volume-notify — desktop notifications for default-sink volume and mute changes.

  Long-running: subscribes to pactl's event stream and re-reads the volume with
  wpctl on each sink event. Sink, app name and timeout are argv.
*/
{
  writeShellApplication,
  wireplumber,
  pulseaudio,
  libnotify,
  gawk,
  coreutils,
}:
writeShellApplication {
  name = "volume-notify";
  runtimeInputs = [
    wireplumber # wpctl
    pulseaudio # pactl
    libnotify # notify-send
    gawk
    coreutils
  ];
  text = builtins.readFile ./volume-notify.sh;
  meta.description = "Notify on PipeWire default-sink volume/mute changes";
}
