# The AP's radio airtime is saturated. Above ~75% utilisation clients stop getting
# airtime and everything wireless goes slow and jittery, while every wired metric on the
# network looks perfectly healthy.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "wifi_channel_saturated";
      title = "WiFi channel saturated";
      expr = "unpoller_device_radio_channel_utilization_total_ratio";
      op = "gt";
      value = 0.75;
      for = "15m";
      severity = "warning";
      category = "unifi";
      noDataState = "OK";
      keepFiringFor = "15m";
      summary = "{{ $labels.name }} radio saturated";
      description = "Channel utilisation on {{ $labels.name }} has been above 75% for 15m. Wireless clients are contending for airtime — expect slowness that no wired metric will explain.";
    })
  ];
}
