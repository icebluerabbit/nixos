# home network
{ den, ... }:
{
  den.aspects.infra._.wlans._.derp = {
    aspect-name = "infra.wlans.derp";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_wlan.derp = {
        name = "derp";
        security = "wpapsk";
        passphrase = "\${var.wifi_passphrase}";
        user_group_id = "6a20c6afaa20ab3db2a1669e";
        network_id = "\${unifi_network.iot.id}";
        wlan_band = "both";
        wlan_bands = [
          "2g"
          "5g"
        ];
        ap_group_ids = [ "6a20c6b0aa20ab3db2a166a3" ];
        group_rekey = 0;
        minimum_data_rate_2g_kbps = 1000;
        minimum_data_rate_5g_kbps = 6000;
        enhanced_iot = true;
        no2ghz_oui = false;
        wpa3_support = false;
        wpa3_transition = false;
        pmf_mode = "disabled";
        bss_transition = false;
        ap_group_mode = "all";
        dtim_6e = 3;
        dtim_mode = "default";
        dtim_na = 3;
        dtim_ng = 1;
        enabled = true;
        fast_roaming_enabled = false;
        hide_ssid = false;
        hotspot2conf_enabled = false;
        iapp_enabled = true;
        is_guest = false;
        l2_isolation = false;
        mac_filter = {
          enabled = false;
          policy = "deny";
        };
        minrate_setting_preference = "auto";
        mlo_enabled = false;
        multicast_enhance = false;
        nas_identifier_type = "bssid";
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
      };

      import = [
        {
          to = "unifi_wlan.derp";
          id = "6a28ce28efa3052c88b2e4fd";
        }
      ];
    };
  };
}
