/*
  vault — unlock/lock the on-demand LUKS2 volume that is absent at boot by design
  https://gitlab.com/cryptsetup/cryptsetup
*/
{
  writeShellApplication,
  cryptsetup,
  util-linux,
  coreutils,
}:
writeShellApplication {
  name = "vault";
  runtimeInputs = [
    cryptsetup
    util-linux
    coreutils
  ];
  text = builtins.readFile ./vault.sh;
  meta.description = "Open, close or inspect the on-demand LUKS vault";
}
