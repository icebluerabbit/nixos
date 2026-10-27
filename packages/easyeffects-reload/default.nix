/*
  easyeffects-reload — load an EasyEffects preset and confirm the filter chain
  came up in the PipeWire graph.
  https://github.com/wwmm/easyeffects

  The easyeffects binary is passed with --easyeffects rather than pinned here:
  the caller is a home-manager service that already resolves its own package.
*/
{
  writeShellApplication,
  pipewire,
  gnugrep,
  coreutils,
}:
writeShellApplication {
  name = "easyeffects-reload";
  runtimeInputs = [
    pipewire
    gnugrep
    coreutils
  ];
  text = builtins.readFile ./easyeffects-reload.sh;
  meta.description = "Load an EasyEffects preset, retrying until the graph node appears";
}
