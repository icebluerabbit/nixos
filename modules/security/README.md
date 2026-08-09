# security

Security tooling — secrets management and related system hardening.

Each aspect configures one security-related concern at the NixOS level. Currently this covers encrypted secrets decrypted at activation time, keeping sensitive values out of the Nix store. New security concerns (firewall rules, kernel hardening, etc.) belong here rather than scattered across host or service modules.
