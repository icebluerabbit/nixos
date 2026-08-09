# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "Generated Dynamic HDR";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "BiTOR";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(BiTOR)$";
        }
      ];
    }
    {
      name = "DepraveD";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(DepraveD)$";
        }
      ];
    }
    {
      name = "Flights";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(Flights)$";
        }
      ];
    }
    {
      name = "GuyZo";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(GuyZo|BR-GuyZo)$";
        }
      ];
    }
    {
      name = "SasukeducK";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(SasukeducK)$";
        }
      ];
    }
    {
      name = "tarunk9c";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(tarunk9c)$";
        }
      ];
    }
    {
      name = "VD0N";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(VD0N)$";
        }
      ];
    }
    {
      name = "VECTOR";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(VECTOR)$";
        }
      ];
    }
    {
      name = "VisionXpert";
      implementation = "ReleaseGroupSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(VisionXpert)$";
        }
      ];
    }
    {
      name = "HDR10+";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\bHDR10(\\+|P(lus)?\\b)";
        }
      ];
    }
    {
      name = "DV";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(dv|dovi|dolby[ .]?v(ision)?)\\b";
        }
      ];
    }
  ];
}
