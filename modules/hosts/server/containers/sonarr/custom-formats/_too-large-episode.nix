_: {
  name = "Too Large (Episode)";
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
          value = 16;
        }
        {
          name = "max";
          value = 9999999;
        }
      ];
    }
    {
      name = "Not Season Pack";
      implementation = "ReleaseTypeSpecification";
      negate = true;
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
