# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "Upscaled";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "AI Upscales";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<=\\b\\d{3,4}p\\b)(?=.*\\b(HEVC)\\b)(?=.*\\b(AI)\\b)";
        }
      ];
    }
    {
      name = "AIUS";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(AIUS)\\b";
        }
      ];
    }
    {
      name = "Regrade";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b((Upscale)?Re-?graded?)\\b";
        }
      ];
    }
    {
      name = "RW";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(RW)\\b";
        }
      ];
    }
    {
      name = "TheUpscaler";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(The[ ._-]?Upscaler)\\b";
        }
      ];
    }
    {
      name = "Upscaled";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<=\\b\\d{3,4}p\\b).*\\b(AI[ ._-]?Enhanced?|UPS(UHD)?|Upscaled?([ ._-]?UHD)?|UpRez)\\b";
        }
      ];
    }
  ];
}
