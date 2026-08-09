# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "3D";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "3D";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<=\\b[12]\\d{3}\\b).*\\b(3d|sbs|half[ .-]ou|half[ .-]sbs)\\b";
        }
      ];
    }
    {
      name = "BluRay3D";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(BluRay3D)\\b";
        }
      ];
    }
    {
      name = "BD3D";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(BD3D)\\b";
        }
      ];
    }
  ];
}
