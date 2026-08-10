# 🔌 Core Networking Services

This directory manages basic system-wide network stack configurations that do not tie into the logical infrastructure definitions.

## Aspects

- **`nat.nix`**: Configures host-level NAT (Network Address Translation) forwarding.
- **`networkmanager.nix`**: Configures the system-wide NetworkManager daemon with unresolved interfaces ignored (e.g., ignoring container virtual bridges `ve-*` and `br-*`).
