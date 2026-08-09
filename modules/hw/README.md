# hw

Hardware configuration — reusable aspects for specific devices and hardware subsystems.

Two kinds of modules live here: subsystem aspects that configure a class of hardware (audio stack, Bluetooth, graphics acceleration, RGB control, sensors, memory tuning), and product aspects that configure a specific peripheral or component (a particular GPU, CPU, keyboard, mouse, headset, monitor, cooler, SSD, or RAM kit). Product aspects typically pull in the relevant subsystem aspects via `includes` and add device-specific udev rules, service config, or RGB profiles on top.
