package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// PlaceholderSecret is what config.yaml used to ship with. Treated as "unset".
const PlaceholderSecret = "CHANGE_ME_32_CHARS_MINIMUM"

type Config struct {
	Server   Server   `mapstructure:"server"`
	Database Database `mapstructure:"database"`
	Auth     Auth     `mapstructure:"auth"`
	Weather  Weather  `mapstructure:"weather"`
	Calendar Calendar `mapstructure:"calendar"`
	Notes    Notes    `mapstructure:"notes"`
	Photos   Photos   `mapstructure:"photos"`
	Devices  Devices  `mapstructure:"devices"`
}

type Server struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	// AllowedOrigins is only consulted in dev, where Vite serves on another port.
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	SecureCookies  bool     `mapstructure:"secure_cookies"`
}

type Database struct {
	Path            string `mapstructure:"path"`
	BackupDir       string `mapstructure:"backup_dir"`
	BackupRetention int    `mapstructure:"backup_retention_days"`
	BackupCron      string `mapstructure:"backup_cron"`
}

type Auth struct {
	JWTSecret string        `mapstructure:"jwt_secret"`
	TokenTTL  time.Duration `mapstructure:"token_ttl"`
}

type Weather struct {
	Latitude  float64       `mapstructure:"latitude"`
	Longitude float64       `mapstructure:"longitude"`
	Timezone  string        `mapstructure:"timezone"`
	CacheTTL  time.Duration `mapstructure:"cache_ttl"`
}

type Calendar struct {
	ICSDir        string `mapstructure:"ics_dir"`
	Watch         bool   `mapstructure:"watch"`
	LookaheadDays int    `mapstructure:"lookahead_days"`
}

type Notes struct {
	Dir   string `mapstructure:"dir"`
	Watch bool   `mapstructure:"watch"`
}

type Photos struct {
	Dir string `mapstructure:"dir"`
}

type Devices struct {
	CheckInterval time.Duration  `mapstructure:"check_interval"`
	Timeout       time.Duration  `mapstructure:"timeout"`
	Targets       []DeviceTarget `mapstructure:"targets"`
}

type DeviceTarget struct {
	Name string `mapstructure:"name" json:"name"`
	Type string `mapstructure:"type" json:"type"`
	// URL is what the health check probes; it is often an API path that is
	// useless to a human.
	URL string `mapstructure:"url" json:"url,omitempty"`
	// Link is where the tile takes you when tapped — the actual web UI.
	Link         string `mapstructure:"link" json:"link,omitempty"`
	Host         string `mapstructure:"host" json:"host,omitempty"`
	Port         int    `mapstructure:"port" json:"port,omitempty"`
	ExpectStatus int    `mapstructure:"expect_status" json:"expect_status,omitempty"`
}

// Load reads config.yaml and then lets a small set of environment variables win.
// Viper does not expand "${VAR}" inside YAML, so secrets are bound explicitly
// rather than written into the file.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "15s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.secure_cookies", false)
	v.SetDefault("database.path", "/app/data/db.sqlite")
	v.SetDefault("database.backup_dir", "/app/data/backup")
	v.SetDefault("database.backup_retention_days", 7)
	v.SetDefault("database.backup_cron", "0 3 * * *")
	v.SetDefault("auth.token_ttl", "168h")
	v.SetDefault("weather.latitude", 48.2082)
	v.SetDefault("weather.longitude", 16.3738)
	v.SetDefault("weather.timezone", "Europe/Vienna")
	v.SetDefault("weather.cache_ttl", "10m")
	v.SetDefault("calendar.ics_dir", "/app/data/ics")
	v.SetDefault("calendar.watch", true)
	v.SetDefault("calendar.lookahead_days", 60)
	v.SetDefault("notes.dir", "/app/data/notes")
	v.SetDefault("notes.watch", true)
	v.SetDefault("photos.dir", "/app/data/photos")
	v.SetDefault("devices.check_interval", "30s")
	v.SetDefault("devices.timeout", "5s")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	// Explicit bindings: nested keys are never reachable via AutomaticEnv.
	for key, env := range map[string]string{
		"auth.jwt_secret":       "JWT_SECRET",
		"weather.latitude":      "WEATHER_LAT",
		"weather.longitude":     "WEATHER_LON",
		"weather.timezone":      "WEATHER_TZ",
		"database.path":         "DB_PATH",
		"server.port":           "SERVER_PORT",
		"server.secure_cookies": "SECURE_COOKIES",
	} {
		if err := v.BindEnv(key, env); err != nil {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, cfg.validate()
}

func (c *Config) validate() error {
	secret := strings.TrimSpace(c.Auth.JWTSecret)
	switch {
	case secret == "", strings.HasPrefix(secret, "${"):
		return fmt.Errorf("JWT_SECRET ist nicht gesetzt — bitte in .env eintragen (openssl rand -base64 48)")
	case strings.HasPrefix(secret, PlaceholderSecret):
		return fmt.Errorf("JWT_SECRET hat noch den Platzhalter-Wert — bitte ändern (openssl rand -base64 48)")
	case len(secret) < 32:
		return fmt.Errorf("JWT_SECRET ist zu kurz (%d Zeichen, mindestens 32)", len(secret))
	}

	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port %d ist ungültig", c.Server.Port)
	}
	if c.Weather.CacheTTL <= 0 {
		return fmt.Errorf("weather.cache_ttl muss > 0 sein")
	}
	if c.Devices.CheckInterval <= 0 {
		return fmt.Errorf("devices.check_interval muss > 0 sein")
	}
	if c.Auth.TokenTTL <= 0 {
		return fmt.Errorf("auth.token_ttl muss > 0 sein")
	}
	return nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
