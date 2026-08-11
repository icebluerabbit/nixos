/*
  openrgb-init — set an OpenRGB device to a fixed colour at boot, with the
  enumeration retry loop every caller needs.
  https://gitlab.com/CalcProgrammer1/OpenRGB

  Device selection, zones, colours and timing are all argv, so one binary
  serves the mousepad, the fan controller and the DDR5 sticks. The openrgb
  binary itself is passed with --openrgb rather than pinned here, so the caller
  keeps using its own package.
*/
{
  writeShellApplication,
  gawk,
  gnugrep,
  coreutils,
}:
writeShellApplication {
  name = "openrgb-init";
  runtimeInputs = [
    gawk
    gnugrep
    coreutils
  ];
  text = builtins.readFile ./openrgb-init.sh;
  meta.description = "Set an OpenRGB device to a fixed colour at boot";
}
