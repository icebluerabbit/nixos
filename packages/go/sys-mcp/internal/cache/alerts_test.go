package cache

import (
	"encoding/json"
	"testing"
)

// Trimmed from the real provisioned output of
// config.containers.grafana.config.services.grafana.provision.alerting.
const alertFixture = `[
  {
    "name": "media",
    "rules": [
      {
        "uid": "qbt_disk_queue",
        "title": "qBittorrent disk queue backed up",
        "for": "15m",
        "noDataState": "OK",
        "labels": {"severity": "warning", "category": "qbittorrent"},
        "annotations": {
          "summary": "qBittorrent disk queue time is high",
          "description": "the write path is backing up"
        },
        "data": [
          {
            "datasourceUid": "PBFA97CFB590B2093",
            "model": {
              "expr": "qbittorrent_global_average_time_queue{host=\"qbittorrent\"}",
              "refId": "A"
            }
          },
          {
            "datasourceUid": "__expr__",
            "model": {
              "conditions": [
                {"evaluator": {"params": [100000], "type": "gt"}}
              ],
              "refId": "B"
            }
          }
        ]
      }
    ]
  }
]`

func TestFlattenAlertGroupsExtractsThreshold(t *testing.T) {
	var groups []rawGroup
	if err := json.Unmarshal([]byte(alertFixture), &groups); err != nil {
		t.Fatal(err)
	}

	rules := FlattenAlertGroups(groups)
	if len(rules) != 1 {
		t.Fatalf("got %d rules, want 1", len(rules))
	}
	rule := rules[0]

	// The whole point of reading evaluated config rather than the alert
	// aspect source: the exact number that defines "bad".
	if rule.Threshold != 100000 {
		t.Errorf("threshold = %v, want 100000", rule.Threshold)
	}
	if rule.Op != "gt" {
		t.Errorf("op = %q, want gt", rule.Op)
	}
	if rule.For != "15m" {
		t.Errorf("for = %q, want 15m", rule.For)
	}
	if rule.Group != "media" {
		t.Errorf("group = %q, want media", rule.Group)
	}
	if rule.Severity != "warning" {
		t.Errorf("severity = %q, want warning", rule.Severity)
	}
	if rule.Category != "qbittorrent" {
		t.Errorf("category = %q, want qbittorrent", rule.Category)
	}

	// The A query's expression, not the __expr__ threshold stage.
	want := "qbittorrent_global_average_time_queue"
	if len(rule.Metrics) != 1 || rule.Metrics[0] != want {
		t.Errorf("metrics = %v, want [%s]", rule.Metrics, want)
	}
	if rule.DatasourceUID != "PBFA97CFB590B2093" {
		t.Errorf("datasourceUid = %q", rule.DatasourceUID)
	}
}

func TestAlertsCacheWithDataSkipsEval(t *testing.T) {
	seeded := []AlertRule{{UID: "seeded"}}
	rules, err := NewAlertsCacheWithData(seeded).Get()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].UID != "seeded" {
		t.Errorf("got %v, want the seeded rule", rules)
	}
}
