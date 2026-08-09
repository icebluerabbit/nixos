# Freeleech gate: match any of the tracker's freeleech indexer-flag tiers.
# Lidarr's IndexerFlags enum is NOT Radarr's: Freeleech=1, Halfleech=2,
# Freeleech75=32, Freeleech25=64 (Radarr uses 256/512 for the partial tiers).
# Values taken from GET /api/v1/customformat/schema on the live instance.
# Non-required specs = OR (match any tier). Scored +100 in the Any profile.
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
          value = 32;
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
          value = 64;
        }
      ];
    }
  ];
}
