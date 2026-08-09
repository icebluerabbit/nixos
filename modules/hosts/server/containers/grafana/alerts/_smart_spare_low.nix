# Spare blocks are down to the threshold the drive itself declares. Compared against
# the drive's own reported threshold rather than a hardcoded number, because what counts
# as "low" is a property of the model.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "smart_spare_low";
      title = "SMART spare blocks low";
      expr = "smartctl_device_available_spare - smartctl_device_available_spare_threshold";
      op = "lt";
      value = 0;
      for = "15m";
      severity = "critical";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.device }} on {{ $labels.host }} is out of spare blocks";
      description = "Available spare blocks on {{ $labels.device }} ({{ $labels.model_name }}) have fallen below the drive's own threshold. It has run out of room to remap failures — replace it.";
    })
  ];
}
