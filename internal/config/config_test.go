package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadAndValidate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("interval = '2s'\nperiod = '5m'\nmetric = 'net_rx'\nascii = true\n[colors]\nup = '#ff0000'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if c.IntervalDuration() != 2*time.Second || c.Period != "5m" || c.Metric != "net_rx" || !c.ASCII || c.RawDays != 7 {
		t.Fatalf("config: %+v", c)
	}
	for _, content := range []string{"interval = '0s'", "interval = '1ms'", "raw_days = -1", "history_days = 0", "period = '2m'", "metric = 'wat'", "unknown = true", "[colors]\nup = 'invalid'"} {
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(p); err == nil {
			t.Errorf("accepted %q", content)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("explicit missing path accepted")
	}
}

func TestSecondPeriodsAreSelectable(t *testing.T) {
	c := Default()
	for _, period := range []string{"1s", "5s", "15s", "1m", "5m", "15m", "1h", "1d"} {
		c.Period = period
		if err := c.Validate(); err != nil {
			t.Fatalf("%s rejected: %v", period, err)
		}
	}
}

func TestOverviewAndChartViewConfiguration(t *testing.T) {
	c := Default()
	if c.View != "overview" {
		t.Fatal("default view is not overview")
	}
	for _, view := range []string{"overview", "chart"} {
		c.View = view
		if err := c.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	c.View = "web"
	if err := c.Validate(); err == nil {
		t.Fatal("unknown view accepted")
	}
}

func TestWindowsInterfaceNamesAndPlatformDataPaths(t *testing.T) {
	c := Default()
	c.Interfaces = []string{"Ethernet 2", "vEthernet (Default Switch)"}
	c.Disks = []string{"PhysicalDrive0"}
	if err := c.Validate(); err != nil {
		t.Fatal("Windows adapter names rejected", err)
	}
	for _, tc := range []struct{ platform, xdg, local, config, home, expected string }{
		{"windows", "", "C:/Users/test/AppData/Local", "ignored", "C:/Users/test", "C:/Users/test/AppData/Local/syskline/history.db"},
		{"darwin", "", "ignored", "/home/test/Library/Application Support", "/home/test", "/home/test/Library/Application Support/syskline/history.db"},
		{"linux", "", "ignored", "ignored", "/home/test", "/home/test/.local/share/syskline/history.db"},
		{"windows", "/custom/data", "ignored", "ignored", "ignored", "/custom/data/syskline/history.db"},
	} {
		if got := filepath.ToSlash(dataPath(tc.platform, tc.xdg, tc.local, tc.config, tc.home)); got != tc.expected {
			t.Fatalf("%s: %s", tc.platform, got)
		}
	}
}
