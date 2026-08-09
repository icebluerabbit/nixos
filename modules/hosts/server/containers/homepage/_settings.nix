# _settings.nix — Homepage `settings` block: chrome, tabs and per-group layout.
# The stylix base16 palette + restyle come from the container aspect's CSS; this
# only sets structural/behavioural options.
_: {
  title = "server.internal";
  description = "Homelab command center";
  theme = "dark";
  color = "slate"; # remapped to the stylix palette by custom.css
  headerStyle = "boxedWidgets";
  statusStyle = "dot";
  target = "_blank";
  hideVersion = true;
  disableUpdateCheck = true;
  useEqualHeights = true;
  hideErrors = true; # optional widgets without a key/endpoint stay quiet

  quicklaunch = {
    searchDescriptions = true;
    hideInternetSearch = false;
    showSearchSuggestions = true;
    provider = "google";
  };

  # Single scrolling page — no tabs. Groups render top-to-bottom in the order
  # they're assembled (Overview → Media → … → Network), each with its own
  # column layout.
  layout = {
    "System Vitals" = {
      style = "row";
      columns = 4;
    };
    "Gateways & Alerts" = {
      style = "row";
      columns = 4;
    };
    "Media" = {
      style = "row";
      columns = 4;
    };
    "Downloads & Indexers" = {
      style = "row";
      columns = 3;
    };
    "Home Automation" = {
      style = "row";
      columns = 3;
    };
    "Infrastructure" = {
      style = "row";
      columns = 3;
    };
    "Observability" = {
      style = "row";
      columns = 3;
    };
    "Network" = {
      style = "row";
      columns = 3;
    };
  };
}
