# Freeleech-Only — clone of "Any" but 720p disallowed, min_format_score = 1
# and the Freeleech CF scored +100, so list titles only auto-grab on freeleech.
_: {
  name = "Freeleech-Only";
  upgrade_allowed = true;
  cutoff = 19;
  min_format_score = 1;
  cutoff_format_score = 0;
  min_upgrade_format_score = 1;
  format_items = [
    {
      format = "\${ref.custom_format.Freeleech}";
      score = 100;
    }
    {
      format = "\${ref.custom_format.3D}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.AV1}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Bad Dual Groups}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.BR-DISK}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Black and White Editions}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Extras}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Generated Dynamic HDR}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.LQ}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.LQ (Release Title)}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Sing-Along Versions}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Upscaled}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Too Large}";
      score = -10000;
    }
  ];
  language = {
    id = -2;
    name = "Original";
  };
  items = [
    {
      allowed = false;
      quality = {
        id = 0;
        name = "Unknown";
        source = "unknown";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 24;
        name = "WORKPRINT";
        source = "workprint";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 25;
        name = "CAM";
        source = "cam";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 26;
        name = "TELESYNC";
        source = "telesync";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 27;
        name = "TELECINE";
        source = "telecine";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 29;
        name = "REGIONAL";
        source = "dvd";
        resolution = 480;
        modifier = "regional";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 28;
        name = "DVDSCR";
        source = "dvd";
        resolution = 480;
        modifier = "screener";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 1;
        name = "SDTV";
        source = "tv";
        resolution = 480;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 2;
        name = "DVD";
        source = "dvd";
        resolution = 0;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 23;
        name = "DVD-R";
        source = "dvd";
        resolution = 480;
        modifier = "remux";
      };
      items = [ ];
    }
    {
      allowed = false;
      name = "WEB 480p";
      id = 1000;
      items = [
        {
          allowed = false;
          quality = {
            id = 8;
            name = "WEBDL-480p";
            source = "webdl";
            resolution = 480;
            modifier = "none";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 12;
            name = "WEBRip-480p";
            source = "webrip";
            resolution = 480;
            modifier = "none";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = false;
      quality = {
        id = 20;
        name = "Bluray-480p";
        source = "bluray";
        resolution = 480;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 21;
        name = "Bluray-576p";
        source = "bluray";
        resolution = 576;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 30;
        name = "Remux-1080p";
        source = "bluray";
        resolution = 1080;
        modifier = "remux";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 31;
        name = "Remux-2160p";
        source = "bluray";
        resolution = 2160;
        modifier = "remux";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 22;
        name = "BR-DISK";
        source = "bluray";
        resolution = 1080;
        modifier = "brdisk";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 10;
        name = "Raw-HD";
        source = "tv";
        resolution = 1080;
        modifier = "rawhd";
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 4;
        name = "HDTV-720p";
        source = "tv";
        resolution = 720;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = false;
      name = "WEB-720p";
      id = 1003;
      items = [
        {
          allowed = false;
          quality = {
            id = 14;
            name = "WEBRip-720p";
            source = "webrip";
            resolution = 720;
            modifier = "none";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 5;
            name = "WEBDL-720p";
            source = "webdl";
            resolution = 720;
            modifier = "none";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = false;
      quality = {
        id = 6;
        name = "Bluray-720p";
        source = "bluray";
        resolution = 720;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = true;
      quality = {
        id = 9;
        name = "HDTV-1080p";
        source = "tv";
        resolution = 1080;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = true;
      name = "WEB-1080p";
      id = 1002;
      items = [
        {
          allowed = true;
          quality = {
            id = 15;
            name = "WEBRip-1080p";
            source = "webrip";
            resolution = 1080;
            modifier = "none";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 3;
            name = "WEBDL-1080p";
            source = "webdl";
            resolution = 1080;
            modifier = "none";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      quality = {
        id = 7;
        name = "Bluray-1080p";
        source = "bluray";
        resolution = 1080;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = true;
      quality = {
        id = 16;
        name = "HDTV-2160p";
        source = "tv";
        resolution = 2160;
        modifier = "none";
      };
      items = [ ];
    }
    {
      allowed = true;
      name = "WEB-2160p";
      id = 1001;
      items = [
        {
          allowed = true;
          quality = {
            id = 17;
            name = "WEBRip-2160p";
            source = "webrip";
            resolution = 2160;
            modifier = "none";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 18;
            name = "WEBDL-2160p";
            source = "webdl";
            resolution = 2160;
            modifier = "none";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      quality = {
        id = 19;
        name = "Bluray-2160p";
        source = "bluray";
        resolution = 2160;
        modifier = "none";
      };
      items = [ ];
    }
  ];
}
