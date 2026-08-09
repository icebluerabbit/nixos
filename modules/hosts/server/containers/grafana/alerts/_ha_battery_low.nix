# A battery-powered Home Assistant device is about to go dark. Unlike a mains device
# dropping off, this one is predictable and silent: the sensor keeps reporting correctly
# right up until it doesn't, and ha_device_unavailable only notices afterwards.
#
# min_over_time, not the raw gauge, and that is a correctness fix rather than smoothing.
# HA's Prometheus exporter is event-driven — it writes a battery value only when the
# reading changes, which for these sensors is roughly every 25 minutes. Prometheus marks
# a series stale after 5m, so an instant query against the bare gauge returns an empty
# vector most of the time and the rule would evaluate to NoData on almost every run.
# Taking the minimum over an hour spans the gaps.
#
# Battery percentages are genuinely numeric, so this domain does not suffer the
# string-state problem documented at length in ha_device_unavailable.
{ promRule, ... }:
{
  group = "home";
  rules = [
    (promRule {
      uid = "ha_battery_low";
      title = "Home Assistant battery low";
      expr = "min_over_time(homeassistant_sensor_battery_percent[1h])";
      op = "lt";
      value = 15;
      lookback = 3600; # must cover the 1h range selector
      for = "1h";
      severity = "warning";
      category = "home";
      noDataState = "OK";
      summary = "{{ $labels.friendly_name }} battery at {{ $value }}%";
      description = "{{ $labels.friendly_name }} ({{ $labels.entity }}) is below 15% battery. Replace it before it drops off entirely — once it does, the only signal is the device going unavailable.";
    })
  ];
}
