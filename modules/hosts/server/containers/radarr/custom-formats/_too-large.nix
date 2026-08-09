_: {
  name = "Too Large";
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
          value = 30;
        }
        {
          name = "max";
          value = 9999999;
        }
      ];
    }
  ];
}
