/*
  logind — systemd login manager; configures seat management, power key handling, and idle inhibition
  https://www.freedesktop.org/software/systemd/man/logind.conf.html
*/
_: {
  den.aspects.services._.logind = {
    aspect-name = "services.logind";
    nixos = {
      services.logind.settings.Login.HandlePowerKey = "poweroff";
    };
  };
}
