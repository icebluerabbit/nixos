# NixOS Configuration

[![CI](https://github.com/icebluerabbit/nixos/actions/workflows/ci.yml/badge.svg)](https://github.com/icebluerabbit/nixos/actions/workflows/ci.yml)
[![Release PR](https://github.com/icebluerabbit/nixos/actions/workflows/promote.yml/badge.svg)](https://github.com/icebluerabbit/nixos/actions/workflows/promote.yml)
[![Framework: Den](https://img.shields.io/badge/framework-den-blue.svg?logo=nixos&logoColor=white)](https://github.com/denful/den)
[![Formatter: nixfmt](https://img.shields.io/badge/formatter-nixfmt-orange.svg)](https://github.com/NixOS/nixfmt)

A highly modular NixOS configuration repository targeting desktop, server, and virtualized environments. 

This flake is built on a **dendritic aspect-oriented structure**, focusing on small, fully self-contained, modular, and shareable aspects. It leverages [den](https://github.com/denful/den) as the underlying configuration framework.

> [!NOTE]
> **System & Network Co-Orchestration (`denful-networks`)**
> This repository is not just a NixOS host configuration; it is the single source of truth for the physical and logical network infrastructure of the homelab. Using the `denful-networks` schema, network definitions (VLANs, static IPs, MAC bindings, and port profiles) are compiled into two outputs to keep the host and the physical switch/router in sync:
> 1. **NixOS Configurations**: Automatic creation of host bridges, VLAN interfaces, firewall rules, and internal nameserver records.
> 2. **Terranix/Terraform Configs**: Generated declarative UniFi profiles (client definitions, subnet rules, port overrides) and Cloudflare DNS zone records.

---

## 📂 Repository Structure

The configuration is structured as independent modules, each containing distinct aspects that package system options, home-manager environments, and dependencies together:

| Module Category | Description |
| :--- | :--- |
| 🖥️ **[apps](modules/apps/README.md)** | GUI applications (Chromium, Neovide, Wezterm, etc.) and their user configurations. |
| 🥾 **[boot](modules/boot/README.md)** | Bootloaders, systemd-initrd configurations, and Disko declarations. |
| 🐚 **[cli](modules/cli/README.md)** | Command-line utilities, shells (Zsh), terminal multiplexers (Tmux), and git configurations. |
| 📦 **[containers](modules/containers/README.md)** | Docker and OCI container services (media stacks, databases, self-hosted applications). |
| 🛠️ **[dev](modules/dev/README.md)** | Developer environments, language servers (LSPs), runtimes, and compiler setups. |
| 🔤 **[fonts](modules/fonts/README.md)** | System-wide custom font packages and rendering settings. |
| 👥 **[groups](modules/groups/README.md)** | User groups and shared access roles declarations. |
| 🖼️ **[gui](modules/gui/README.md)** | Compositor configuration (Niri), bar modules (Quickshell), and notification daemons (Mako). |
| 💻 **[hosts](modules/hosts/README.md)** | Machine-specific host profiles (`desktop`, `server`) and hardware configurations. |
| 🔌 **[hw](modules/hw/README.md)** | Hardware-specific drivers (Nvidia, AMD), controllers (liquidctl, coolercontrol), and peripherals. |
| 🌐 **[infra](modules/infra/README.md)** | Network infrastructure definitions, static IP allocations, VLAN networks, device specifications, and cloudflare/unifi integrations. |
| 💿 **[iso](modules/iso/README.md)** | Custom minimal installation ISO generation rules and automated setup scripts. |
| 🔌 **[networking](modules/networking/README.md)** | Core system-wide network services (NAT, NetworkManager daemon configurations). |
| 🔒 **[security](modules/security/README.md)** | Secret management setup utilizing SOPS-NIX and age keys. |
| ⚙️ **[services](modules/services/README.md)** | System daemons (SSH, resolving, sd-login, Btrfs scrubbing utilities). |
| 🎨 **[style](modules/style/README.md)** | Theme and system styling setup using Stylix (base16 templates). |
| 👤 **[users](modules/users/README.md)** | User account definitions and individual home-manager profiles. |

---

## 🔍 sys-mcp — Repo-Aware MCP Server

[`packages/sys-mcp/`](packages/sys-mcp/) is a Model Context Protocol server bundled with this repo that understands the homelab's topology. It is declared in [`modules/cli/mcp.nix`](modules/cli/mcp.nix) and automatically registered with all MCP-aware clients (Claude Code, IDEs, agents) via home-manager's `programs.mcp` — no manual wiring required.

**Capabilities:**

- **Nix topology** — resolve containers, VLANs, infra clients, and physical devices; look up anything by IP, port, or fuzzy name; browse the full network topology
- **Runtime** — fetch container logs, systemd status, and exec commands inside containers (auto-SSHes to the correct host)
- **UniFi live state** — query active devices, connected clients, networks, WiFi SSIDs, firewall policies, and port forward/profile rules directly from the controller

Credentials and addresses are injected automatically: the UniFi URL is resolved from the infra device definition, and the API key is read from SOPS at runtime.

---

## 🛠️ Justfile Commands

We use a `Justfile` to organize common administration, building, and maintenance operations.

### Building & Deploying
* **`just build`**: Builds both the server and desktop configurations (dry run).
* **`just build-desktop`**: Builds the desktop configuration locally without applying changes (dry run).
* **`just build-server`**: Builds the server configuration locally without applying changes (dry run).
* **`just test`**: Tests both the server and desktop configurations in-place temporarily.
* **`just test-desktop`**: Test desktop changes in place temporarily (reverts on reboot).
* **`just test-server`**: Test server changes in place temporarily (reverts on reboot).
* **`just switch`**: Apply the entire homelab configuration in sequence (runs network infrastructure apply, server deploy, and desktop apply).
* **`just switch-desktop`**: Apply changes immediately and switch the local desktop system.
* **`just switch-server`**: Deploy the NixOS configuration to the remote server host.
* **`just boot-desktop`**: Apply desktop changes and write them to the bootloader (active on next reboot).

### Infrastructure Orchestration
* **`just infra-plan-homelab`**: Preview declarative UniFi and Cloudflare configuration changes.
* **`just infra-apply-homelab`**: Deploy declarative UniFi and Cloudflare configuration changes.

### Secrets Management
* **`just sops-user-secrets`**: Edit user-level secrets (`secrets/user-secrets.yaml`) using SOPS.
* **`just sops-host-secrets`**: Edit host-level secrets (`secrets/host-secrets.yaml`) as root using the host's AGE key.

### Flake Maintenance
* **`just flake`**: Regenerates the `flake.nix` output mappings using `flake-file` to register new modules.
* **`just update`**: Performs a `nix flake update` to bump all input dependencies.
* **`just update-den`**: Updates specifically the `den` framework dependency.
* **`just format`**: Formats all Nix source files recursively.

### VM Testing & Installer Development
* **`just vm-desktop`**: Launches the `desktop` configuration in a virtual machine for local testing.
* **`just vm-server`**: Launches the `server` configuration in a virtual machine.
* **`just make-iso [out]`**: Builds the custom installation ISO image (defaults output to `/tmp/nixos-installer`).
* **`just test-iso [out] [key]`**: Boots the generated installer ISO in QEMU to test partitioning and automated installs.

### System Cleanup
* **`just clean`**: Garbage collects the Nix store using root privileges.
* **`just gc`**: Runs `nh clean all` to prune old generations and optimize storage.

---

## 🚀 Getting Started

1. **Enter the environment**:
   If you have `direnv` enabled, entering the directory will automatically initialize the environment and install git hooks. Otherwise, run:
   ```console
   nix develop
   ```
2. **Apply changes**:
   Modify your settings, regenerate the flake configuration if you added new modules, and switch:
   ```console
   just flake
   just switch-desktop
   ```
