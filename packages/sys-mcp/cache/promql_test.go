package cache

import (
	"reflect"
	"testing"
)

func TestMetricNamesInDropsFunctionsAndLabels(t *testing.T) {
	cases := []struct {
		name string
		expr string
		want []string
	}{
		{
			// The case that motivated the builtin list: two tokens match
			// the regex and only one is a metric.
			name: "range function wrapper",
			expr: "min_over_time(homeassistant_sensor_battery_percent[1h])",
			want: []string{"homeassistant_sensor_battery_percent"},
		},
		{
			// Label matcher values look exactly like metric names.
			name: "label matcher stripped",
			expr: `qbittorrent_global_average_time_queue{host="qbittorrent"}`,
			want: []string{"qbittorrent_global_average_time_queue"},
		},
		{
			name: "nested subquery with two metrics",
			expr: "avg_over_time((qbt_peers_uploading and on() " +
				"(rate(node_pressure_io_stalled_seconds_total[5m]) > 0.2))[7d:5m])",
			want: []string{
				"node_pressure_io_stalled_seconds_total",
				"qbt_peers_uploading",
			},
		},
		{
			name: "empty expression",
			expr: "",
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MetricNamesIn(tc.expr)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("MetricNamesIn(%q) = %v, want %v",
					tc.expr, got, tc.want)
			}
		})
	}
}
