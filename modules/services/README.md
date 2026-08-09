# services

System daemons and background services — things that run continuously or on a schedule.

Each aspect enables and configures one systemd service or system-level daemon: display manager, DNS resolver, login manager, power management, filesystem maintenance, remote access, and application backends. These are NixOS-level concerns configured via `nixos` modules rather than home-manager, since they run as system services rather than user processes.
