# Rejects foreign-language releases (scored -10000 in both profiles). Aimed at
# DigitalCore, which carries heavy German/French/Italian/Spanish dubs, but it
# applies library-wide so it also covers the RSS-sync path that autobrr filters
# never see.
#
# All four specs are required = true, so they AND: the format matches only when a
# release is NONE of original / English / Korean / Japanese. A bare negated
# "Original" spec would have been wrong — Sonarr parses an untagged release as
# English, so every English-subbed release of a Japanese-original anime (80 series
# here) would have scored as "not original" and been rejected.
#
# Known consequence: a MULTi release (e.g. GERMAN.DL) carries English alongside
# German, so "Not English" fails and it is allowed through. It does contain English
# audio, so that is intended — but it is the opposite of what a per-language
# deny-list would do.
_: {
  name = "Not Original Language";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "Not Original";
      implementation = "LanguageSpecification";
      negate = true;
      required = true;
      fields = [
        {
          name = "value";
          value = -2;
        }
      ];
    }
    {
      name = "Not English";
      implementation = "LanguageSpecification";
      negate = true;
      required = true;
      fields = [
        {
          name = "value";
          value = 1;
        }
      ];
    }
    {
      name = "Not Korean";
      implementation = "LanguageSpecification";
      negate = true;
      required = true;
      fields = [
        {
          name = "value";
          value = 21;
        }
      ];
    }
    {
      name = "Not Japanese";
      implementation = "LanguageSpecification";
      negate = true;
      required = true;
      fields = [
        {
          name = "value";
          value = 8;
        }
      ];
    }
  ];
}
