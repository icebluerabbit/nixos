package config

import (
	"testing"
	"time"
)

func TestParseDefaults(t *testing.T) {
	c, err := Parse(nil)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
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
	c, err := Parse([]string{
		"--listen", "127.0.0.1",
		"--port", "9000",
		"--mam-seed-path", "/run/secrets/mam",
		"--accept-media-type", "Ebook",
		"--accept-category-allow", "Ebooks - Fantasy",
		"--accept-category-allow", "Audiobooks - Horror",
		"--accept-size-min", "1024",
		"--grab-dry-run=false",
		"--grab-inflight-window", "10m",
		"--client-category", "Ebook=books",
		"--client-category", "Audiobook=audiobooks",
		"--wedge-enable",
		"--wedge-proactive-media-type", "Audiobook",
		"--webhook-header", "X-Token=abc",
	})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if c.Listen != "127.0.0.1" || c.Port != "9000" {
		t.Errorf("addr = %s:%s", c.Listen, c.Port)
	}
	if c.MAM.SeedPath != "/run/secrets/mam" {
		t.Errorf("seed-path = %q", c.MAM.SeedPath)
	}
	if len(c.Accept.MediaTypes) != 1 || c.Accept.MediaTypes[0] != "Ebook" {
		t.Errorf("media-types = %v", c.Accept.MediaTypes)
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
	if c.Client.Categories["Ebook"] != "books" || c.Client.Categories["Audiobook"] != "audiobooks" {
		t.Errorf("client categories = %v", c.Client.Categories)
	}
	if !c.Wedge.Enable {
		t.Error("wedge-enable should be true")
	}
	if len(c.Wedge.ProactiveMediaTypes) != 1 || c.Wedge.ProactiveMediaTypes[0] != "Audiobook" {
		t.Errorf("proactive-media-types = %v", c.Wedge.ProactiveMediaTypes)
	}
	if c.Webhook.Headers["X-Token"] != "abc" {
		t.Errorf("webhook headers = %v", c.Webhook.Headers)
	}
}
