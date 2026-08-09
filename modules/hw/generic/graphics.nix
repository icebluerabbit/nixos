# graphics — hardware graphics subsystem with OpenGL/Vulkan acceleration and DRI support
_: {
  den.aspects.hw._.graphics = {
    aspect-name = "hw.graphics";
    nixos = {
      hardware.graphics = {
        enable = true;
        enable32Bit = true;
      };

      hardware.opengl = {
        # enable = true;
        # driSupport = true;
        # driSupport32Bit = true;
      };
    };
  };
}
