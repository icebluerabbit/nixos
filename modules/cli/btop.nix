/*
  btop — resource monitor for CPU, memory, network, and GPU with tree process view
  https://github.com/aristocratos/btop
*/
_: {
  den.aspects.cli._.btop = {
    aspect-name = "cli.btop";
    homeManager = _: {
      programs.btop = {
        enable = true;
        settings = {
          theme_background = false;
          rounded_corners = true;
          shown_boxes = "proc cpu mem net gpu0";
          update_ms = 500;
          proc_sorting = "cpu direct";
          proc_tree = true;
          proc_gradient = true;
          proc_aggregate = true;
          cpu_bottom = true;
          temp_scale = "fahrenheit";
          mem_below_net = true;
          show_battery = false;
          save_config_on_exit = false;
        };
      };
    };
  };
}
