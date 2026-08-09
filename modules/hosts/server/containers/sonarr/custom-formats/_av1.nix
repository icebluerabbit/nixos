# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "AV1";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "AV1";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "value";
          value = "\\bAV1\\b";
        }
      ];
    }
  ];
}
