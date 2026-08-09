# A UniFi device is cooking. The UDM Pro Max reports several thermal zones (board,
# CPU, HDD bays) under `temp_area`, so the alert names which one.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "unifi_device_hot";
      title = "UniFi device hot";
      expr = "unpoller_device_temperature_celsius";
      op = "gt";
      value = 80;
      for = "10m";
      severity = "warning";
      category = "unifi";
      noDataState = "OK";
      summary = "{{ $labels.name }} at {{ $labels.temp_area }} above 80°C";
      description = "{{ $labels.name }} has held above 80°C at {{ $labels.temp_area }} for 10m — check airflow and fan level.";
    })
  ];
}
