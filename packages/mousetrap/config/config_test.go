package config

import (
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	c := Parse(nil)
	if c.Listen != "0.0.0.0" || c.Port != "8080" {
		t.Errorf("addr defaults = %s:%s", c.Listen, c.Port)
	}
	if c.MAM.APIBase != "https://www.myanonamouse.net" {
		t.Errorf("mam-api-base default = %q", c.MAM.APIBase)
	}
	if !c.Grab.DryRun {
		t.Error("grab-dry-run should default true")
	}
	if c.Grab.InFlightWindow != 30*time.Minute {
		t.Errorf("inflight-window default = %v", c.Grab.InFlightWindow)
	}
	if c.Grab.UnsatMargin != 5 {
		t.Errorf("unsat-margin default = %d", c.Grab.UnsatMargin)
	}
	if !c.Shop.DryRun {
		t.Error("shop-dry-run should default true")
	}
	if c.Shop.VIPCost != 5000 {
		t.Errorf("shop-vip-cost default = %d", c.Shop.VIPCost)
	}
}

func TestParseOverrides(t *testing.T) {
	c := Parse([]string{
		"--listen", "127.0.0.1",
		"--port", "9000",
		"--mam-seed-path", "/run/secrets/mam",
		"--accept-kind", "ebook",
		"--accept-category-allow", "Ebooks - Fantasy",
		"--accept-category-allow", "Audiobooks - Horror",
		"--accept-size-min", "1024",
		"--grab-dry-run=false",
		"--grab-inflight-window", "10m",
		"--client-category", "ebook=books",
		"--client-category", "audiobook=audiobooks",
		"--wedge-enable",
		"--wedge-proactive-kind", "audiobook",
		"--webhook-header", "X-Token=abc",
	})

	if c.Listen != "127.0.0.1" || c.Port != "9000" {
		t.Errorf("addr = %s:%s", c.Listen, c.Port)
	}
	if c.MAM.SeedPath != "/run/secrets/mam" {
		t.Errorf("seed-path = %q", c.MAM.SeedPath)
	}
	if len(c.Accept.Kinds) != 1 || c.Accept.Kinds[0] != "ebook" {
		t.Errorf("kinds = %v", c.Accept.Kinds)
	}
	if len(c.Accept.CategoriesAllow) != 2 {
		t.Errorf("category-allow = %v", c.Accept.CategoriesAllow)
	}
	if c.Accept.SizeMinBytes != 1024 {
		t.Errorf("size-min = %d", c.Accept.SizeMinBytes)
	}
	if c.Grab.DryRun {
		t.Error("grab-dry-run should be false")
	}
	if c.Grab.InFlightWindow != 10*time.Minute {
		t.Errorf("inflight-window = %v", c.Grab.InFlightWindow)
	}
	if c.Client.Categories["ebook"] != "books" || c.Client.Categories["audiobook"] != "audiobooks" {
		t.Errorf("client categories = %v", c.Client.Categories)
	}
	if !c.Wedge.Enable {
		t.Error("wedge-enable should be true")
	}
	if len(c.Wedge.ProactiveKinds) != 1 || c.Wedge.ProactiveKinds[0] != "audiobook" {
		t.Errorf("proactive-kinds = %v", c.Wedge.ProactiveKinds)
	}
	if c.Webhook.Headers["X-Token"] != "abc" {
		t.Errorf("webhook headers = %v", c.Webhook.Headers)
	}
}
