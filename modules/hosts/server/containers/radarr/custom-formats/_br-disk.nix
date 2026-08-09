# TRaSH "BR-DISK" (trash_id 85c61753df5da1fb2aab6f2a47426b09), default score -10000.
# Blocks full BluRay/HD-DVD disc rips (BDMV/ISO/COMPLETE/BD25-100) at grab time.
_: {
  name = "BR-DISK";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "BR-DISK";
      implementation = "ReleaseTitleSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "value";
          value = "^(?!.*\\b((?<!HD[._ -]|HD)DVD|BDRip|720p|MKV|XviD|WMV|d3g|(BD)?REMUX|^(?=.*1080p)(?=.*HEVC)|[xh][-_. ]?26[45]|German.*[DM]L|((?<=\\d{4}).*German.*([DM]L)?)(?=.*\\b(AVC|HEVC|VC[-_. ]?1|MVC|MPEG[-_. ]?2)\\b))\\b)(((?=.*\\b(Blu[-_. ]?ray|BD|HD[-_. ]?DVD)\\b)(?=.*\\b(AVC|HEVC|VC[-_. ]?1|MVC|MPEG[-_. ]?2|BDMV|ISO)\\b))|^((?=.*\\b(((?=.*\\b((.*_)?COMPLETE.*|Dis[ck])\\b)(?=.*(Blu[-_. ]?ray|HD[-_. ]?DVD)))|3D[-_. ]?BD|BR[-_. ]?DISK|Full[-_. ]?Blu[-_. ]?ray|^((?=.*((BD|UHD)[-_. ]?(25|50|66|100|ISO)))))))).*";
        }
      ];
    }
  ];
}
