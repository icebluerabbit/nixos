_: {
  name = "Too Large (Season)";
  include_custom_format_when_renaming = false;
  specifications = [
    {
      name = "Too Large";
      implementation = "SizeSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "min";
          value = 60;
        }
        {
          name = "max";
          value = 9999999;
        }
      ];
    }
    {
      name = "Season Pack";
      implementation = "ReleaseTypeSpecification";
      negate = false;
      required = true;
      fields = [
        {
          name = "value";
          value = 3;
        }
      ];
    }
  ];
}
