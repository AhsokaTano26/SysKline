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
