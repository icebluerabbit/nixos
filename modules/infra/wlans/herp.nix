# home network
{ den, ... }:
{
  den.aspects.infra._.wlans._.herp = {
    aspect-name = "infra.wlans.herp";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_wlan.herp = {
        name = "herp";
        security = "wpapsk";
        passphrase = "\${var.wifi_passphrase}";
        user_group_id = "6a20c6afaa20ab3db2a1669e";
        network_id = "\${unifi_network.trust.id}";
        wlan_bands = [
          "5g"
        ];
        ap_group_ids = [ "6a20c6b0aa20ab3db2a166a3" ];
        group_rekey = 0;
        minimum_data_rate_2g_kbps = 1000;
        minimum_data_rate_5g_kbps = 6000;
        wpa3_support = true;
        wpa3_transition = true;
        pmf_mode = "optional";
        bss_transition = true;
        ap_group_mode = "all";
        dtim_6e = 3;
        dtim_mode = "default";
        dtim_na = 3;
        dtim_ng = 1;
        enabled = true;
        enhanced_iot = false;
        fast_roaming_enabled = false;
        hide_ssid = false;
        hotspot2conf_enabled = false;
        iapp_enabled = true;
        is_guest = false;
        l2_isolation = false;
        mac_filter = {
          enabled = false;
          policy = "allow";
        };
        minrate_setting_preference = "auto";
        mlo_enabled = false;
        multicast_enhance = false;
        nas_identifier_type = "bssid";
        no2ghz_oui = true;
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

        lifecycle = {
          ignore_changes = [ "wlan_band" ];
        };
      };

      import = [
        {
          to = "unifi_wlan.herp";
          id = "6a28ce03efa3052c88b2e4f9";
        }
      ];
    };
  };
}
