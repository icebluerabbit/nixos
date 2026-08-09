# Dumped verbatim from the live arr (was recyclarr-managed via TRaSH).
_: {
  name = "LQ (Release Title)";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "1XBET";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(1XBET)\\b";
        }
      ];
    }
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
      name = "D3US";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(-D3US|D3US-)";
        }
      ];
    }
    {
      name = "EVO (no WEBDL)";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<=\\b[12]\\d{3}\\b.*?)(?<!\\b(web[ ._-]?(dl|rip)?).*?)\\b(EVO)\\b";
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
      name = "GalaxyRG";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(GalaxyRG)\\b";
        }
      ];
    }
    {
      name = "HHWEB (No MA)";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "^(?!.*\\bMA\\b.*\\bWEB-?DL\\b).*\\b(HHWEB)\\b";
        }
      ];
    }
    {
      name = "jennaortega";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<!-)\\b(jennaortega(UHD)?)\\b";
        }
      ];
    }
    {
      name = "PiRaTeS (no WEBDL)";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<=\\b[12]\\d{3}\\b.*?)(?<!\\b(web[ ._-]?(dl|rip)?).*?)\\b(PiRaTeS)\\b";
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
      name = "READ NOTE";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(READ(\\s|\\.)+NOTE)\\b";
        }
      ];
    }
    {
      name = "SWTYBLZ";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(SWTYBLZ)\\b";
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
    {
      name = "TEKNO3D";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(TEKNO3D)\\b";
        }
      ];
    }
    {
      name = "UnKn0wn (NoRemux)";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "(?<!\\b(remux).*?)\\b(unkn0wn)\\b";
        }
      ];
    }
    {
      name = "Will1869";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = false;
      fields = [
        {
          name = "value";
          value = "\\b(Will1869)\\b";
        }
      ];
    }
  ];
}
