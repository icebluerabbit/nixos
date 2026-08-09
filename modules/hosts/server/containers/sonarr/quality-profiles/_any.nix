# "Any" profile — dumped verbatim from live (previously recyclarr-managed).
# Manual adds use this: curated HD tiers, junk CFs at -10000, min score 0.
_: {
  name = "Any";
  upgrade_allowed = true;
  cutoff = 19;
  min_format_score = 0;
  cutoff_format_score = 0;
  min_upgrade_format_score = 1;
  format_items = [
    {
      # Prefer freeleech at grab time. Any still grabs anything (min_format_score
      # = 0); with cutoff_format_score = 0 a non-freeleech grab already meets the
      # cutoff, so this only breaks ties toward free — no re-grab/upgrade churn.
      format = "\${ref.custom_format.Freeleech}";
      score = 100;
    }
    {
      format = "\${ref.custom_format.AV1}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Not Original Language}";
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
      format = "\${ref.custom_format.Extras}";
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
      format = "\${ref.custom_format.Upscaled}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Too Large (Episode)}";
      score = -10000;
    }
    {
      format = "\${ref.custom_format.Too Large (Season)}";
      score = -10000;
    }
  ];
  items = [
    {
      # Sonarr requires the Unknown tier present, but its "unknown" source is a
      # decode-only enum variant configuratarr can't encode — so omit source here.
      allowed = false;
      quality = {
        id = 0;
        name = "Unknown";
        resolution = 0;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 1;
        name = "SDTV";
        source = "television";
        resolution = 480;
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
            id = 12;
            name = "WEBRip-480p";
            source = "webRip";
            resolution = 480;
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 8;
            name = "WEBDL-480p";
            source = "web";
            resolution = 480;
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = false;
      quality = {
        id = 2;
        name = "DVD";
        source = "dvd";
        resolution = 480;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 13;
        name = "Bluray-480p";
        source = "bluray";
        resolution = 480;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 22;
        name = "Bluray-576p";
        source = "bluray";
        resolution = 576;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 10;
        name = "Raw-HD";
        source = "televisionRaw";
        resolution = 1080;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 20;
        name = "Bluray-1080p Remux";
        source = "blurayRaw";
        resolution = 1080;
      };
      items = [ ];
    }
    {
      allowed = false;
      quality = {
        id = 21;
        name = "Bluray-2160p Remux";
        source = "blurayRaw";
        resolution = 2160;
      };
      items = [ ];
    }
    {
      allowed = true;
      quality = {
        id = 4;
        name = "HDTV-720p";
        source = "television";
        resolution = 720;
      };
      items = [ ];
    }
    {
      allowed = true;
      name = "WEB-720p";
      id = 1003;
      items = [
        {
          allowed = true;
          quality = {
            id = 14;
            name = "WEBRip-720p";
            source = "webRip";
            resolution = 720;
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 5;
            name = "WEBDL-720p";
            source = "web";
            resolution = 720;
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      quality = {
        id = 6;
        name = "Bluray-720p";
        source = "bluray";
        resolution = 720;
      };
      items = [ ];
    }
    {
      allowed = true;
      quality = {
        id = 9;
        name = "HDTV-1080p";
        source = "television";
        resolution = 1080;
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
            source = "webRip";
            resolution = 1080;
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 3;
            name = "WEBDL-1080p";
            source = "web";
            resolution = 1080;
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
      };
      items = [ ];
    }
    {
      allowed = true;
      quality = {
        id = 16;
        name = "HDTV-2160p";
        source = "television";
        resolution = 2160;
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
            source = "webRip";
            resolution = 2160;
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 18;
            name = "WEBDL-2160p";
            source = "web";
            resolution = 2160;
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
      };
      items = [ ];
    }
  ];
}
