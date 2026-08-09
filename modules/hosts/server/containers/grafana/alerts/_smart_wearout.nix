# Write endurance consumed. Not urgent at 80% — this is the one disk alert meant to
# arrive months early, while replacing the drive is a purchase rather than a recovery.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "smart_wearout";
      title = "SMART wearout";
      expr = "smartctl_device_percentage_used";
      op = "gt";
      value = 80;
      for = "1h";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.device }} on {{ $labels.host }} is {{ $value }}% worn";
      description = "{{ $labels.device }} ({{ $labels.model_name }}) on {{ $labels.host }} has used more than 80% of its rated write endurance. Still healthy — start planning the replacement.";
    })
  ];
}
