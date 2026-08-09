# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "LQ (Release Title)";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "BEN THE MEN";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(BEN[ ._-]THE[ ._-]MEN)\\b";
        }
      ];
    }
    {
      name = "BiTOR (2160p)";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?=.*?(\\b2160p\\b))(?=.*?(\\bBiTOR\\b))";
        }
      ];
    }
    {
      name = "CREATiVE24";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(CREATiVE24)\\b";
        }
      ];
    }
    {
      name = "Feranki1980";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(Feranki1980)\\b";
        }
      ];
    }
    {
      name = "R&H";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(R&H)\\b";
        }
      ];
    }
    {
      name = "TeeWee";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(TeeWee)\\b";
        }
      ];
    }
  ];
}
