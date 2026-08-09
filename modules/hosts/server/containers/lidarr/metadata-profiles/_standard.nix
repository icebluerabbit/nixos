# "Standard" metadata profile — the sole profile every artist uses. Based on Lidarr's
# built-in id 1, widened from its studio-albums-only default.
#
# This decides what "entire artist" means: the Spotify list monitors each followed
# artist's whole catalogue, and these toggles are what that catalogue is drawn from.
#   primary:   Album, EP, Single          (not Broadcast, Other)
#   secondary: Studio, Soundtrack, Compilation
#              (not Live, Remix, DJ-mix, Demo, Mixtape, Spokenword, Interview, Audio drama)
#   status:    Official, Promotion        (not Bootleg, Pseudo-Release)
#
# The server always sends all three toggle lists in full; only `allowed` is ours.
_: {
  name = "Standard";
  primary_album_types = [
    {
      allowed = true;
      album_type = {
        id = 2;
        name = "Single";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 4;
        name = "Other";
      };
    }
    {
      allowed = true;
      album_type = {
        id = 1;
        name = "EP";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 3;
        name = "Broadcast";
      };
    }
    {
      allowed = true;
      album_type = {
        id = 0;
        name = "Album";
      };
    }
  ];
  secondary_album_types = [
    {
      allowed = true;
      album_type = {
        id = 0;
        name = "Studio";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 3;
        name = "Spokenword";
      };
    }
    {
      allowed = true;
      album_type = {
        id = 2;
        name = "Soundtrack";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 7;
        name = "Remix";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 9;
        name = "Mixtape/Street";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 6;
        name = "Live";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 4;
        name = "Interview";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 8;
        name = "DJ-mix";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 10;
        name = "Demo";
      };
    }
    {
      allowed = true;
      album_type = {
        id = 1;
        name = "Compilation";
      };
    }
    {
      allowed = false;
      album_type = {
        id = 11;
        name = "Audio drama";
      };
    }
  ];
  release_statuses = [
    {
      allowed = false;
      release_status = {
        id = 3;
        name = "Pseudo-Release";
      };
    }
    {
      allowed = true;
      release_status = {
        id = 1;
        name = "Promotion";
      };
    }
    {
      allowed = true;
      release_status = {
        id = 0;
        name = "Official";
      };
    }
    {
      allowed = false;
      release_status = {
        id = 2;
        name = "Bootleg";
      };
    }
  ];
}
