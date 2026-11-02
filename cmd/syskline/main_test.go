package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDemoSnapshotAndIsolation(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	var out, stderr bytes.Buffer
	if err := run([]string{"--demo", "--once"}, &out, &stderr); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Mode    string `json:"mode"`
		Metrics map[string]struct {
			Value float64 `json:"value"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(out.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value.Mode != "demo" || len(value.Metrics) != 10 || value.Metrics["memory"].Value <= 0 {
		t.Fatalf("snapshot: %s", out.String())
	}
	entries, err := os.ReadDir(data)
	if err != nil || len(entries) != 0 {
		t.Fatal("demo wrote production data", entries, err)
	}
}

func TestCLIValidationPrecedenceAndHelp(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("interval = '2s'\nascii = false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	o, err := parse([]string{"--config", p, "--interval", "500ms", "--ascii"}, &bytes.Buffer{})
	if err != nil || o.cfg.Interval != "500ms" || !o.cfg.ASCII {
		t.Fatalf("override: %+v %v", o, err)
	}
	for _, args := range [][]string{{"--interval", "0s"}, {"--period", "2m"}, {"--wat"}, {"unexpected"}, {"--config", "/missing/syskline"}, {"--demo", "--db", "production.db"}} {
		if err := run(args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Errorf("accepted %v", args)
		}
	}
	for _, args := range [][]string{{"--help"}, {"--version"}} {
		var out bytes.Buffer
		if err := run(args, &out, &bytes.Buffer{}); err != nil || out.Len() == 0 {
			t.Fatalf("%v: %v %s", args, err, out.String())
		}
	}
}

func TestViewFlag(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, view := range []string{"overview", "chart"} {
		o, err := parse([]string{"--view", view}, &bytes.Buffer{})
		if err != nil || o.cfg.View != view {
			t.Fatalf("view %s: %v", view, err)
		}
	}
	if _, err := parse([]string{"--view", "web"}, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown view accepted")
	}
}
