# home network
#
# The LACP trunk to the server (USW Pro HD ports 25+26 → bond0).
#
# Every VLAN the server bridges, and MGMT deliberately excluded. bond0 holds no address
# itself — each `vlan-*` subinterface does — and there is no vlan-mgmt among them: the
# server reaches MGMT on its own NIC (enp9s0, 10.10.10.2) via Pro HD port 2. So VLAN 10
# on this trunk is delivered only to be discarded, and it is the path that carried an
# unknown-unicast NFS flood at bond0.
#
# The list must track hosts/server/networking/vlans/*.nix. Add a VLAN there and forget it
# here and the bridge comes up with no traffic on it.
#
# native stays Default (untagged) — unused by the server, but it is what the port carried
# implicitly before this profile existed, so leaving it there changes nothing.
{ den, ... }:
(import ../_mk-port-profile.nix { inherit den; }) {
  id = "server-trunk";
  resourceName = "server_trunk";
  displayName = "Server Trunk";
  network = "default";
  unifiId = "6a791cfdabfc0160bad62c2d";
  extra = {
    # Restriction is a DENYLIST. tagged_networkconf_ids (the allowlist model) is NOT
    # implemented by this controller — it discards the list and reports back
    # forward="all". The controller only honours tagged_vlan_mgmt="custom" +
    # excluded_networkconf_ids, and it then *derives* forward="customize" from the
    # presence of exclusions. All three fields must be stated together and must agree
    # with what the controller computes, or the provider dies with "Provider produced
    # inconsistent result after apply".
    #
    # Excluding mgmt means every other VLAN stays tagged, so this list does NOT need
    # touching when a VLAN is added — unlike an allowlist, which would silently drop
    # the new one.
    tagged_vlan_mgmt = "custom";
    excluded_networkconf_ids = [
      "\${unifi_network.mgmt.id}"
    ];

    # Stated explicitly to match Living Room Trunk. Port 25's override also sets this,
    # but leaving the profile silent means STP on the LACP trunk rides on
    # override-beats-profile precedence rather than on the profile saying what it
    # means. Storm control is deliberately absent: the server is a legitimate bulk
    # receiver (NFS from the UNAS), and clamping unicast here would throttle it.
    stp_port_mode = true;
  };
}
