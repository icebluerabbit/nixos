# _info.nix — top information bar (Homepage `widgets`).
# greeting (left) · weather · clock · search · UniFi summary (right-aligned).
{ gw, ... }:
[
  {
    greeting = {
      text_size = "xl";
      text = "Welcome home";
    };
  }
  {
    openmeteo = {
      label = "Home";
      latitude = "{{HOMEPAGE_VAR_LATITUDE}}";
      longitude = "{{HOMEPAGE_VAR_LONGITUDE}}";
      timezone = "America/New_York";
      units = "imperial";
      cache = 5;
    };
  }
  {
    datetime = {
      text_size = "l";
      format = {
        dateStyle = "long";
        timeStyle = "short";
        hourCycle = "h23";
      };
    };
  }
  {
    search = {
      provider = "google";
      showSearchSuggestions = true;
      target = "_blank";
    };
  }
  {
    unifi_console = {
      url = "https://${gw}";
      username = "{{HOMEPAGE_VAR_UNIFI_CONTROLLER_USERNAME}}";
      password = "{{HOMEPAGE_VAR_UNIFI_CONTROLLER_PASSWORD}}";
    };
  }
]
