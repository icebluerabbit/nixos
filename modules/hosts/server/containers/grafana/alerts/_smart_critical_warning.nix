# The drive has raised its own critical_warning flag — the controller reporting trouble,
# not an inference from a counter. Zero on healthy hardware. The flag is a bitmask:
# 0x1 spare low, 0x2 temperature, 0x4 reliability degraded, 0x8 read-only, 0x10 backup
# failed. PromQL has no bitwise op, but the only temperature-ONLY value is exactly 2 — any
# other nonzero value carries a serious bit. So temp (2) is a warning (check cooling), and
# everything else is critical (plan replacement). A hot drive that ALSO has a serious bit
# reads !=2, so it correctly fires critical, not the temp warning.
{ promRule, ... }:
{
  group = "infra";
  rules = [
    (promRule {
      uid = "smart_critical_warning";
      title = "SMART critical warning";
      expr = "smartctl_device_critical_warning > 0 and smartctl_device_critical_warning != 2";
      op = "gt";
      value = 0;
      for = "5m";
      severity = "critical";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.device }} on {{ $labels.host }} raised a critical warning";
      description = "The drive controller for {{ $labels.device }} ({{ $labels.model_name }}) on {{ $labels.host }} set a critical_warning bit — spare low, reliability degraded, read-only, or backup failed (not temperature). Back up anything that matters and plan a replacement.";
    })
    (promRule {
      uid = "smart_temp_warning";
      title = "SMART drive over temperature";
      expr = "smartctl_device_critical_warning == 2";
      op = "gt";
      value = 0;
      for = "10m";
      severity = "warning";
      category = "storage";
      noDataState = "OK";
      summary = "{{ $labels.device }} on {{ $labels.host }} over temperature threshold";
      description = "The drive controller for {{ $labels.device }} ({{ $labels.model_name }}) on {{ $labels.host }} set only the temperature warning bit — it is running hot, not failing. Check airflow/cooling (case cover, heatsink, obstruction). Clears on its own once it cools.";
    })
  ];
}
