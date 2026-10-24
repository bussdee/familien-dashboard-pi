package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const kopf = "auth:\n  jwt_secret: \"0123456789012345678901234567890123456789\"\n"

// laden schreibt eine config.yaml und liest sie wie der Server.
func laden(t *testing.T, yaml string) (*Config, error) {
	t.Helper()
	t.Setenv("JWT_SECRET", "")
	t.Setenv("NTFY_URL", "")
	t.Setenv("NTFY_TOKEN", "")
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	return Load(p)
}

func TestVoreinstellungen(t *testing.T) {
	cfg, err := laden(t, kopf)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.ProxyHops != 2 {
		t.Errorf("proxy_hops erwartet 2, war %d", cfg.Server.ProxyHops)
	}
	if cfg.Calendar.LookaheadDays != 400 {
		t.Errorf("lookahead_days erwartet 400, war %d", cfg.Calendar.LookaheadDays)
	}
	if cfg.Notify.NtfyURL != "" {
		t.Errorf("Benachrichtigungen müssen standardmäßig aus sein, war %q", cfg.Notify.NtfyURL)
	}
	d := cfg.Weather.DryWindow
	if d.WetFromMM != 0.1 || d.WetFromProbability != 55 || d.MinHours != 1.5 || d.SunnyBelowCloud != 40 {
		t.Errorf("Trockenfenster-Voreinstellung weicht von den bisherigen Werten ab: %+v", d)
	}
}

func TestTrockenfensterAusDerDatei(t *testing.T) {
	cfg, err := laden(t, kopf+"weather:\n  dry_window:\n    wet_from_probability: 65\n    min_hours: 2\n")
	if err != nil {
		t.Fatal(err)
	}
	d := cfg.Weather.DryWindow
	if d.WetFromProbability != 65 || d.MinHours != 2 || d.WetFromMM != 0.1 {
		t.Errorf("Werte nicht übernommen oder Rest nicht aufgefüllt: %+v", d)
	}
}

func TestUngueltigeTrockenfensterWerte(t *testing.T) {
	for _, zeile := range []string{
		"wet_from_mm: 0", "wet_from_mm: 9", "wet_from_probability: 0", "wet_from_probability: 120",
		"min_hours: 0.1", "min_hours: 12", "sunny_below_cloud: 0", "sunny_below_cloud: 101",
	} {
		_, err := laden(t, kopf+"weather:\n  dry_window:\n    "+zeile+"\n")
		if err == nil || !strings.Contains(err.Error(), "dry_window") {
			t.Errorf("%s: erwartet Fehler, war %v", zeile, err)
		}
	}
}

func TestProxyStufenGrenzen(t *testing.T) {
	if _, err := laden(t, kopf+"server:\n  proxy_hops: 9\n"); err == nil {
		t.Error("proxy_hops 9 sollte abgelehnt werden")
	}
	if cfg, err := laden(t, kopf+"server:\n  proxy_hops: 0\n"); err != nil || cfg.Server.ProxyHops != 0 {
		t.Errorf("proxy_hops 0 ist erlaubt (kein Proxy): %v", err)
	}
}

func TestNtfyAusDerUmgebung(t *testing.T) {
	t.Setenv("NTFY_URL", "https://ntfy.example.com/familie")
	t.Setenv("NTFY_TOKEN", "tk_x")
	p := filepath.Join(t.TempDir(), "config.yaml")
	_ = os.WriteFile(p, []byte(kopf), 0o600)
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Notify.NtfyURL != "https://ntfy.example.com/familie" || cfg.Notify.NtfyToken != "tk_x" {
		t.Errorf("Umgebung nicht gelesen: %+v", cfg.Notify)
	}
}

func TestJWTSecretPflicht(t *testing.T) {
	for name, yaml := range map[string]string{
		"fehlt":       "server:\n  port: 8080\n",
		"Platzhalter": "auth:\n  jwt_secret: \"CHANGE_ME_32_CHARS_MINIMUM_xxxxxxxxxxxxxxxx\"\n",
		"zu kurz":     "auth:\n  jwt_secret: \"kurz\"\n",
		"${VAR}":      "auth:\n  jwt_secret: \"${JWT_SECRET}\"\n",
	} {
		if _, err := laden(t, yaml); err == nil {
			t.Errorf("%s: erwartet Fehler", name)
		}
	}
}
