/*
  coolercontrol-init-config — copy a coolercontrold config into /etc before the
  daemon starts.
  https://gitlab.com/coolercontrol/coolercontrol

  coolercontrold rewrites its config at runtime, so it cannot be a store
  symlink; source, destination and modes are argv.
*/
{
  writeShellApplication,
  coreutils,
}:
writeShellApplication {
  name = "coolercontrol-init-config";
  runtimeInputs = [ coreutils ];
  text = builtins.readFile ./coolercontrol-init-config.sh;
  meta.description = "Install a coolercontrold config file before the daemon starts";
}
