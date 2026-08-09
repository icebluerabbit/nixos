# _network.nix — Network tab: UniFi controller, NAS, and the physical fabric.
{
  cl,
  gw,
  ...
}:
let
  # Device link card → UniFi controller (pings across the mgmt subnet from the
  # apps VLAN would false-negative, so these are informational links).
  dev =
    {
      name,
      icon,
      model,
    }:
    {
      "${name}" = {
        inherit icon;
        href = "https://${gw}/";
        description = model;
      };
    };
in
[
  {
    "Network" = [
      {
        "UniFi Controller" = {
          icon = "unifi.svg";
          href = "https://${gw}/";
          widget = {
            type = "unifi";
            url = "https://${gw}";
            username = "{{HOMEPAGE_VAR_UNIFI_CONTROLLER_USERNAME}}";
            password = "{{HOMEPAGE_VAR_UNIFI_CONTROLLER_PASSWORD}}";
            fields = [
              "uptime"
              "wan"
              "lan_users"
              "wlan_users"
            ];
          };
        };
      }
      {
        "UNAS Pro" = {
          icon = "unifi-drive.svg";
          href = "https://${cl.unas-pro.ip}/";
          widget = {
            type = "unifi_drive";
            url = "https://${cl.unas-pro.ip}";
            username = "{{HOMEPAGE_VAR_UNIFI_DRIVE_USERNAME}}";
            password = "{{HOMEPAGE_VAR_UNIFI_DRIVE_PASSWORD}}";
            fields = [
              "status"
              "total"
              "used"
              "available"
            ];
          };
        };
      }
      (dev {
        name = "Dream Machine Pro Max";
        icon = "mdi-router-network-#ff79c6";
        model = "UDM Pro Max — gateway / controller";
      })
      (dev {
        name = "U7 Pro XG";
        icon = "mdi-access-point-#8be9fd";
        model = "WiFi 7 access point";
      })
      (dev {
        name = "USW Pro HD 24 PoE";
        icon = "mdi-switch-#50fa7b";
        model = "24-port PoE core switch";
      })
      (dev {
        name = "USW Flex 2.5G 8";
        icon = "mdi-switch-#f1fa8c";
        model = "8-port living-room switch";
      })
    ];
  }
]
