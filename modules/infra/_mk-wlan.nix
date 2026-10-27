/*
  _mk-wlan.nix — constructor for `infra.wlans.*` aspects (a WiFi SSID).

  Underscore-prefixed so import-tree skips it: this is a function, not a module.

  Defaults describe the modern 5GHz WPA2/WPA3-transition SSID. The IoT network
  overrides most of them: legacy clients there cannot associate with WPA3 or
  management-frame protection, and need the 2.4GHz band.

  `unifiId` is the controller's own object id for the SSID, which is what an
  import has to key on.
*/
{ lib }:
{
  id,
  resourceName,
  ssid,
  network,
  unifiId,
  wlanBands,
  wlanBand ? null,
  wpa3 ? true,
  pmfMode ? "optional",
  bssTransition ? true,
  enhancedIot ? false,
  no2ghzOui ? true,
  macFilterPolicy ? "allow",
  ignoreWlanBandChanges ? false,
}:
{
  mkOs = {
    infra.wlans.${id} = {
      inherit ssid network unifiId;
      bands = wlanBands;
      inherit wpa3;
      hidden = false;
    };
  };

  mkTerranix = {
    resource.unifi_wlan.${resourceName} = {
      name = ssid;
      security = "wpapsk";
      passphrase = "\${var.wifi_passphrase}";
      user_group_id = "6a20c6afaa20ab3db2a1669e";
      network_id = "\${unifi_network.${network}.id}";
      wlan_bands = wlanBands;
      ap_group_ids = [ "6a20c6b0aa20ab3db2a166a3" ];
      group_rekey = 0;
      minimum_data_rate_2g_kbps = 1000;
      minimum_data_rate_5g_kbps = 6000;
      wpa3_support = wpa3;
      wpa3_transition = wpa3;
      pmf_mode = pmfMode;
      bss_transition = bssTransition;
      ap_group_mode = "all";
      dtim_6e = 3;
      dtim_mode = "default";
      dtim_na = 3;
      dtim_ng = 1;
      enabled = true;
      enhanced_iot = enhancedIot;
      fast_roaming_enabled = false;
      hide_ssid = false;
      hotspot2conf_enabled = false;
      iapp_enabled = true;
      is_guest = false;
      l2_isolation = false;
      mac_filter = {
        enabled = false;
        policy = macFilterPolicy;
      };
      minrate_setting_preference = "auto";
      mlo_enabled = false;
      multicast_enhance = false;
      nas_identifier_type = "bssid";
      no2ghz_oui = no2ghzOui;
      private_preshared_keys_enabled = false;
      proxy_arp = false;
      radius_mac_auth_enabled = false;
      site = "default";
      uapsd = false;
      vlan_enabled = false;
      wpa3_enhanced_192 = false;
      wpa3_fast_roaming = false;
      wpa_enc = "ccmp";
      wpa_mode = "wpa2";
    }
    // lib.optionalAttrs (wlanBand != null) { wlan_band = wlanBand; }
    // lib.optionalAttrs ignoreWlanBandChanges {
      lifecycle = {
        ignore_changes = [ "wlan_band" ];
      };
    };

    import = [
      {
        to = "unifi_wlan.${resourceName}";
        id = unifiId;
      }
    ];
  };
}
