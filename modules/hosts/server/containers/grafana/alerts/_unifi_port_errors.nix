# A switch port is corrupting a meaningful fraction of what it receives. Almost always a
# physical fault — a marginal cable, a dirty SFP, a failing NIC — and it shows up as
# retransmits and mystery slowness long before anything actually goes down.
#
# Thresholded as a *ratio*, not a count. Every link has a nonzero error floor, and the
# busiest port in the network (the LAN trunk, ~300M packets/hour) hits it constantly: a
# raw `> 0` fires on a handful of errors per billion packets and cries wolf forever. A
# genuinely degrading link corrupts a stable, traffic-proportional share of its input, so
# the share is the signal and the count is noise. 1e-4 sits ~2 orders above the observed
# floor and well under the ~1e-3 where TCP throughput starts visibly collapsing.
#
# Ports with no traffic divide 0/0 and drop out of the vector — hence noDataState = OK.
{ promRule, ... }:
{
  group = "network";
  rules = [
    (promRule {
      uid = "unifi_port_errors";
      title = "UniFi port errors";
      expr = "rate(unpoller_device_port_receive_errors_total[15m]) / rate(unpoller_device_port_receive_packets_total[15m])";
      op = "gt";
      value = 1.0e-4;
      lookback = 900;
      for = "30m";
      severity = "warning";
      category = "unifi";
      noDataState = "OK";
      keepFiringFor = "10m";
      summary = "RX errors on {{ $labels.name }}";
      description = "{{ $labels.port_name }} on {{ $labels.name }} has been corrupting over 0.01% of received packets for 30m — suspect the cable, the SFP, or the NIC at the far end.";
    })
  ];
}
