/*
  Logitech mouse — logiops logid daemon with smartshift and hi-res scroll config
  https://github.com/PixlOne/logiops
*/
_: {
  den.aspects.hw._.logitech-mouse = {
    aspect-name = "hw.logitech-mouse";
    nixos = _: {
      hardware.logitech.wireless.enable = true;
      hardware.logitech.wireless.enableGraphical = true;
      boot.kernelModules = [ "hid-logitech-hidpp" ];

      services.udev.extraRules = ''
        # Disable USB autosuspend for Logitech Bolt and Unifying receivers to prevent lag/stutter
        ACTION=="add", SUBSYSTEM=="usb", ATTR{idVendor}=="046d", ATTR{idProduct}=="c548", ATTR{power/control}="on"
        ACTION=="add", SUBSYSTEM=="usb", ATTR{idVendor}=="046d", ATTR{idProduct}=="c52b", ATTR{power/control}="on"

        # Force HID++ driver for the Bolt receiver
        ACTION=="add|change", SUBSYSTEM=="hid", KERNEL=="0003:046D:C548.*", ATTR{/sys/bus/hid/drivers/hid-generic/unbind}="%k", ATTR{/sys/bus/hid/drivers/logitech-hidpp-device/bind}="%k"
      '';
    };
  };
}
