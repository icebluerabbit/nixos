# "Any" profile — the sole profile every artist uses; the root folder defaults to it.
# Taken from Lidarr's built-in id 1, with three deliberate departures from the live row:
#   - Trash (MP3-8..MP3-80) and Poor (MP3-96..MP3-160) lossy groups disallowed, and
#     Unknown with them. This is the only quality floor on the music path — autobrr
#     pushes carry no indexer flags for Lidarr (unlike radarr/sonarr), so a
#     score-based gate cannot work here.
#   - Freeleech CF at +100. min_format_score = 0, so nothing is rejected for lacking
#     it; it only breaks ties toward free at grab time.
_: {
  name = "Any";
  # Chase quality upward: every album keeps getting replaced by a better grab until it
  # reaches the cutoff. soularr (search_source = all) feeds Lidarr's cutoff-unmet list,
  # so lossy albums get hunted for a lossless replacement on Soulseek.
  upgrade_allowed = true;
  # Cutoff = Lossless 24bit group (1006): 16-bit lossless counts as cutoff-unmet, so Lidarr
  # keeps chasing a hi-res replacement for every album until a real 24bit source turns up.
  # The 24bit qualities are split into their own group below (id 1006) ranked above the
  # 16-bit Lossless group — Lidarr rejects a cutoff that points at a quality nested inside
  # a group ("Cutoff must be an allowed quality or group"), so the standalone group is
  # required. Lidarr classifies FLAC vs FLAC 24bit from the imported file's actual bit
  # depth, so this gate is on true resolution, not release-name text. Trade-off (accepted):
  # churns and eventually denylists on the many albums with no 24bit source anywhere, and it
  # refills the whole cutoff-unmet list. Set back to 1005 (Lossless group) to stop.
  cutoff = 1006;
  min_format_score = 0;
  cutoff_format_score = 0;
  format_items = [
    {
      format = "\${ref.custom_format.Freeleech}";
      score = 100;
    }
  ];
  items = [
    {
      allowed = false;
      quality = {
        id = 0;
        name = "Unknown";
      };
      items = [ ];
    }
    {
      allowed = false;
      name = "Trash Quality Lossy";
      id = 1000;
      items = [
        {
          allowed = false;
          quality = {
            id = 32;
            name = "MP3-8";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 31;
            name = "MP3-16";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 30;
            name = "MP3-24";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 29;
            name = "MP3-32";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 28;
            name = "MP3-40";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 27;
            name = "MP3-48";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 26;
            name = "MP3-56";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 25;
            name = "MP3-64";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 24;
            name = "MP3-80";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = false;
      name = "Poor Quality Lossy";
      id = 1001;
      items = [
        {
          allowed = false;
          quality = {
            id = 23;
            name = "MP3-96";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 33;
            name = "MP3-112";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 22;
            name = "MP3-128";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 19;
            name = "OGG Vorbis Q5";
          };
          items = [ ];
        }
        {
          allowed = false;
          quality = {
            id = 5;
            name = "MP3-160";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      name = "Low Quality Lossy";
      id = 1002;
      items = [
        {
          allowed = true;
          quality = {
            id = 1;
            name = "MP3-192";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 18;
            name = "OGG Vorbis Q6";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 9;
            name = "AAC-192";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 20;
            name = "WMA";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 34;
            name = "MP3-224";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      name = "Mid Quality Lossy";
      id = 1003;
      items = [
        {
          allowed = true;
          quality = {
            id = 17;
            name = "OGG Vorbis Q7";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 8;
            name = "MP3-VBR-V2";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 3;
            name = "MP3-256";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 16;
            name = "OGG Vorbis Q8";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 10;
            name = "AAC-256";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      name = "High Quality Lossy";
      id = 1004;
      items = [
        {
          allowed = true;
          quality = {
            id = 2;
            name = "MP3-VBR-V0";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 12;
            name = "AAC-VBR";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 4;
            name = "MP3-320";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 15;
            name = "OGG Vorbis Q9";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 11;
            name = "AAC-320";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 14;
            name = "OGG Vorbis Q10";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = true;
      name = "Lossless";
      id = 1005;
      items = [
        {
          allowed = true;
          quality = {
            id = 6;
            name = "FLAC";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 7;
            name = "ALAC";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 35;
            name = "APE";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 36;
            name = "WavPack";
          };
          items = [ ];
        }
      ];
    }
    {
      # Split out of the Lossless group so it can be a valid cutoff target (see cutoff note
      # above) and so it ranks strictly above 16-bit lossless — a 16-bit FLAC album is now an
      # eligible upgrade toward a 24bit source.
      allowed = true;
      name = "Lossless 24bit";
      id = 1006;
      items = [
        {
          allowed = true;
          quality = {
            id = 21;
            name = "FLAC 24bit";
          };
          items = [ ];
        }
        {
          allowed = true;
          quality = {
            id = 37;
            name = "ALAC 24bit";
          };
          items = [ ];
        }
      ];
    }
    {
      allowed = false;
      quality = {
        id = 13;
        name = "WAV";
      };
      items = [ ];
    }
  ];
}
