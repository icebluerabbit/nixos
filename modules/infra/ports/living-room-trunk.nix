# home network
#
# The trunk to the living-room switch (USW Pro HD port 9 ↔ USW Flex port 9).
#
# Tagged VLANs are an explicit allowlist, NOT `all`. Everything behind the Flex reaches
# other subnets by routing through the UDM — Desktop hits both mgmt (10.10.10.2) and the
# UNAS (10.10.50.222) `via 10.10.20.1`, from its TRUST address — so the trunk only needs
# the VLANs devices are *natively* on. Carrying the rest just widens the flood domain:
# an unknown-unicast flood on MGMT was egressing on the Hue Hub and PS5 ports (100 Mbit,
# ~25k pps dropped each) purely because this trunk handed VLAN 10 to a switch where
# nothing is on VLAN 10.
#
# The flip side: Flex ports 5-8 are empty. Plug a device into one on a VLAN that is not
# listed below and the link will come up and pass nothing. Add the VLAN here at the same
# time you add the port_override in infra/devices/usw-flex-2-5g-8.nix.
#
# native stays Default (untagged) — that is the Flex's own management address
# (10.100.218.255). Changing it loses the switch.
#
# The terraform resource address stays `living_room` on purpose. It is the key into TF
# state and carries the `import` below; renaming it without a `moved` block plans a
# destroy + re-import of a profile both trunk ports depend on.
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "living-room-trunk";
    resourceName = "living_room";
    displayName = "Living Room Trunk";
    network = "default";
    unifiId = "6a20cb97119e8dfcb5ece105";
    excludedNetworks = [
      "mgmt"
      "storage"
      "guest"
      "dmz"
      "apps"
      "media"
      "download"
      "services"
    ];
    extra = {
      # Restriction is a DENYLIST. tagged_networkconf_ids (the allowlist model) is NOT
      # implemented by this controller — it discards the list and reports back
      # forward="all". The controller only honours tagged_vlan_mgmt="custom" +
      # excluded_networkconf_ids, and it then *derives* forward="customize" from the
      # presence of exclusions. All three fields must agree with what the controller
      # computes, or the provider dies with "Provider produced inconsistent result
      # after apply". forward and excluded_networkconf_ids come from the helper —
      # the latter off `excludedNetworks` above — so only this flag is stated here.
      #
      # What stays tagged: trust (Desktop, port 4), iot (Hue Hub, port 2),
      # work (MacBook, port 1), game (PlayStation 5, port 3). Nothing else is behind
      # this switch.
      tagged_vlan_mgmt = "custom";

      autoneg = true;
      lldpmed_enabled = true;
      stp_port_mode = true;
      dot1x_ctrl = "force_authorized";
      setting_preference = "auto";
      egress_rate_limit_kbps_enabled = false;
      full_duplex = false;
      isolation = false;
      port_keepalive_enabled = false;
      port_security_enabled = false;
      stormctrl_bcast_enabled = false;
      stormctrl_mcast_enabled = false;
      stormctrl_ucast_enabled = false;
      lldpmed_notify_enabled = false;
    };
  };
in
{
  den.aspects.infra._.ports._.living-room-trunk = {
    aspect-name = "infra.ports.living-room-trunk";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      # `default` is the native VLAN; the rest are the exclusions below, which
      # this profile names as Terraform addresses and so depends on.
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.default
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.guest
      den.aspects.infra._.networks._.media
      den.aspects.infra._.networks._.mgmt
      den.aspects.infra._.networks._.services
      den.aspects.infra._.networks._.storage
    ];

    os = _: port.mkOs;
    terranix = _: port.mkTerranix;
  };
}
