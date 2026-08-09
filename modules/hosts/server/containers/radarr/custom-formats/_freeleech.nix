# Freeleech gate: match any of the tracker's freeleech indexer-flag tiers.
# Radarr enum: Freeleech=1 (100%), Halfleech=2 (50%), Freeleech75=256,
# Freeleech25=512. Non-required specs = OR (match any tier). Scored +100 in
# Freeleech-Only. NOTE: TorrentLeech freeleech is binary — only value 1 ever
# fires there; the partial tiers cover Gazelle-style trackers.
_: {
  name = "Freeleech";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "Freeleech";
      implementation = "IndexerFlagSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = 1;
        }
      ];
    }
    {
      name = "Halfleech";
      implementation = "IndexerFlagSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = 2;
        }
      ];
    }
    {
      name = "Freeleech75";
      implementation = "IndexerFlagSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = 256;
        }
      ];
    }
    {
      name = "Freeleech25";
      implementation = "IndexerFlagSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = 512;
        }
      ];
    }
  ];
}
