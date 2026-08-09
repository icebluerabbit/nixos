# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "Sing-Along Versions";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "Sing-Along";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "value";
          value = "(?<=\\b[12]\\d{3}\\b).*\\b(Sing[-_. ]Along)\\b";
        }
      ];
    }
  ];
}
