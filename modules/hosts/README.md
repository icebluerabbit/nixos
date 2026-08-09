# hosts

Machine-specific NixOS configuration — settings that are particular to a physical host.

Each host has its own subdirectory containing aspects for the concerns that vary per machine: bootloader, filesystem layout and mount options, network interfaces, hardware platform specifics, locale/timezone, and top-level host assembly. Nothing in here is reusable across machines; reusable hardware configuration belongs in `hw/`.
