# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "Extras";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "Extras";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "value";
          value = "(?<=\\bS\\d+\\b).*\\b(Extras|Bonus|Extended[ ._-]Clip)\\b";
        }
      ];
    }
  ];
}
