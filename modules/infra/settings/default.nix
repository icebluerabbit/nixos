# home network site settings
{ den, ... }:
(import ../_mk-setting.nix { inherit den; }) {
  id = "default";
  resourceName = "default";
  settings = {
    doh = {
      server_names = [
        "cloudflare"
        "google"
      ];
      state = "off";
    };

    igmp_snooping = {
      enabled = false;
      network_ids = [ ];
    };

    ips = {
      advanced_filtering_preference = "manual";
      content_filtering_blocking_page_enabled = true;
      enabled_categories = [
        "emerging-malware"
        "emerging-netbios"
        "emerging-dns"
        "emerging-exploit"
        "emerging-pop3"
        "emerging-smtp"
        "emerging-tftp"
        "emerging-voip"
        "emerging-mobile"
        "emerging-dos"
        "emerging-worm"
        "botcc"
        "emerging-sql"
        "malicious-hosts"
        "dshield"
        "emerging-shellcode"
        "emerging-misc"
        "emerging-activex"
        "emerging-imap"
        "emerging-telnet"
        "emerging-attackresponse"
        "emerging-useragent"
        "ciarmy"
        "emerging-rpc"
        "emerging-webclient"
        "emerging-webapps"
        "compromised"
        "emerging-snmp"
        "emerging-webserver"
        "emerging-ftp"
        "emerging-scan"
      ];
      enabled_networks = [ "6a20c6afaa20ab3db2a1669d" ];
      honeypot_enabled = false;
      ips_mode = "ids";
      memory_optimized = true;
    };

    mgmt = {
      auto_upgrade = true;
      ssh_enabled = false;
    };

    usg = {
      broadcast_ping = false;
      ftp_module = true;
      gre_module = true;
      h323_module = true;
      icmp_timeout = 30;
      mss_clamp = "auto";
      offload_accounting = true;
      offload_l2_blocking = true;
      offload_sch = true;
      other_timeout = 600;
      pptp_module = true;
      receive_redirects = false;
      send_redirects = true;
      sip_module = false;
      syn_cookies = true;
      tcp_close_timeout = 10;
      tcp_close_wait_timeout = 60;
      tcp_established_timeout = 7440;
      tcp_fin_wait_timeout = 120;
      tcp_last_ack_timeout = 30;
      tcp_syn_recv_timeout = 60;
      tcp_syn_sent_timeout = 120;
      tcp_time_wait_timeout = 120;
      tftp_module = true;
      timeout_setting_preference = "auto";
      udp_other_timeout = 30;
      udp_stream_timeout = 180;
      upnp_enabled = false;
      upnp_nat_pmp_enabled = true;
      upnp_secure_mode = true;
      dns_verification = {
        setting_preference = "auto";
        primary_dns_server = "1.1.1.1";
        domain = "ui.com";
        secondary_dns_server = "8.8.8.8";
      };
    };
  };
}
