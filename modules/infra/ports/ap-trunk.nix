# home network
#
# The trunk to the U7 Pro XG (USW Pro HD port 1). Previously the port carried a
# name and PoE but no profile, so it forwarded everything by default.
#
# Tagged VLANs are only the ones an SSID actually lands on — see infra/wlans:
# derp → iot, herp → trust, werp → work. Everything else is excluded for the same
# reason as the living-room trunk: a VLAN handed to a switch where nothing is on
# it only widens the flood domain.
#
# The flip side, and it is sharp: add a WLAN on a VLAN not listed here and the
# SSID comes up and passes nothing. Add the network to `excludedNetworks`'
# complement at the same time you add the WLAN aspect.
#
# native stays Default (untagged) — that is the AP's own management address
# (10.100.134.21). Changing it loses the AP.
#
# `unifiId` is the object the first apply created. It only feeds the `import`
# block, so a rebuilt state adopts this profile instead of creating a duplicate.
{ den, ... }:
let
  port = (import ../_mk-port-profile.nix null) {
    id = "ap-trunk";
    resourceName = "ap_trunk";
    displayName = "AP Trunk";
    network = "default";
    unifiId = "6a7f661cabfc0160bad81468";
    excludedNetworks = [
      "mgmt"
      "storage"
      "guest"
      "dmz"
      "apps"
      "media"
      "download"
      "services"
      "game"
    ];
    extra = {
      # Restriction is a DENYLIST — see ports/living-room-trunk.nix for why the
      # allowlist model does not work on this controller. forward and
      # excluded_networkconf_ids come from the helper, so only this flag is here.
      tagged_vlan_mgmt = "custom";

      # Matches the other two trunks.
      stp_port_mode = true;
    };
  };
in
{
  den.aspects.infra._.ports._.ap-trunk = {
    aspect-name = "infra.ports.ap-trunk";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.default
      den.aspects.infra._.networks._.dmz
      den.aspects.infra._.networks._.download
      den.aspects.infra._.networks._.game
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
