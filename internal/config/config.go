package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	DB       DBConfig       `yaml:"db"`
	Security SecurityConfig `yaml:"security"`
	Storage  StorageConfig  `yaml:"storage"`
	App      AppConfig      `yaml:"app"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"` // e.g. ":8443"

	// Defensive defaults for public-facing deployments.
	ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
	ReadTimeout       time.Duration `yaml:"read_timeout"`
	WriteTimeout      time.Duration `yaml:"write_timeout"`
	IdleTimeout       time.Duration `yaml:"idle_timeout"`
	ShutdownTimeout   time.Duration `yaml:"shutdown_timeout"`
	MaxHeaderBytes    int           `yaml:"max_header_bytes"`

	// RequestTimeout is applied via middleware to bound handler execution.
	// Keep it higher than typical DB query timeouts and template rendering.
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type DBConfig struct {
	DSN string `yaml:"dsn"` // e.g. "postgres://user:pass@localhost:5432/blogcms?sslmode=disable"

	// Pool controls (database/sql).
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time"`
	PingTimeout     time.Duration `yaml:"ping_timeout"`
}

type AppConfig struct {
	// MarkdownRendererPool limits concurrent markdown conversions and avoids global locks.
	MarkdownRendererPool int `yaml:"markdown_renderer_pool"`

	// Small in-memory caches to reduce DB chatter on hot pages.
	SettingsCacheTTL time.Duration `yaml:"settings_cache_ttl"`
	TagCloudCacheTTL time.Duration `yaml:"tagcloud_cache_ttl"`
}

type SecurityConfig struct {
	SessionKey   string `yaml:"session_key"`   // used to sign session tokens
	CookieSecure bool   `yaml:"cookie_secure"` // set true behind HTTPS
}

type StorageConfig struct {
	UploadDir     string `yaml:"upload_dir"`      // local filesystem directory for uploaded files
	PublicBaseURL string `yaml:"public_base_url"` // URL prefix to serve uploads (default: "/uploads/")
	MaxUploadMB   int    `yaml:"max_upload_mb"`   // hard limit for upload size
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Addr:              ":8443",
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       2 * time.Minute,
			ShutdownTimeout:   10 * time.Second,
			MaxHeaderBytes:    1 << 20, // 1 MiB
			RequestTimeout:    10 * time.Second,
		},
		DB: DBConfig{
			DSN:             "postgres://blogcms:blogcms@localhost:5432/blogcms?sslmode=disable",
			MaxOpenConns:    50,
			MaxIdleConns:    25,
			ConnMaxLifetime: 30 * time.Minute,
			ConnMaxIdleTime: 5 * time.Minute,
			PingTimeout:     5 * time.Second,
		},
		Security: SecurityConfig{
			SessionKey:   "CHANGE_ME__LONG_RANDOM",
			CookieSecure: false,
		},
		Storage: StorageConfig{
			UploadDir:     "./data/uploads",
			PublicBaseURL: "/uploads/",
			MaxUploadMB:   10,
		},
		App: AppConfig{
			MarkdownRendererPool: 4,
			SettingsCacheTTL:     30 * time.Second,
			TagCloudCacheTTL:     30 * time.Second,
		},
	}
}

// LoadFromPath loads configuration from an optional YAML file path, applies environment overrides,
// and validates required fields.
func LoadFromPath(cfgPath string) (Config, error) {
	cfg := Default()

	if strings.TrimSpace(cfgPath) != "" {
		b, err := os.ReadFile(cfgPath)
		if err != nil {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config: %w", err)
		}
	}

	applyEnv(&cfg)

	if strings.TrimSpace(cfg.DB.DSN) == "" {
		return Config{}, errors.New("db.dsn is required")
	}
	if strings.TrimSpace(cfg.Security.SessionKey) == "" {
		return Config{}, errors.New("security.session_key is required")
	}
	if cfg.Storage.MaxUploadMB <= 0 {
		cfg.Storage.MaxUploadMB = 10
	}
	if strings.TrimSpace(cfg.Storage.PublicBaseURL) == "" {
		cfg.Storage.PublicBaseURL = "/uploads/"
	}
	if !strings.HasSuffix(cfg.Storage.PublicBaseURL, "/") {
		cfg.Storage.PublicBaseURL += "/"
	}
	if strings.TrimSpace(cfg.Storage.UploadDir) == "" {
		cfg.Storage.UploadDir = "./data/uploads"
	}

	// Server/App sanity.
	if cfg.Server.ReadHeaderTimeout <= 0 {
		cfg.Server.ReadHeaderTimeout = 5 * time.Second
	}
	if cfg.Server.ReadTimeout <= 0 {
		cfg.Server.ReadTimeout = 15 * time.Second
	}
	if cfg.Server.WriteTimeout <= 0 {
		cfg.Server.WriteTimeout = 30 * time.Second
	}
	if cfg.Server.IdleTimeout <= 0 {
		cfg.Server.IdleTimeout = 2 * time.Minute
	}
	if cfg.Server.ShutdownTimeout <= 0 {
		cfg.Server.ShutdownTimeout = 10 * time.Second
	}
	if cfg.Server.MaxHeaderBytes <= 0 {
		cfg.Server.MaxHeaderBytes = 1 << 20
	}
	if cfg.Server.RequestTimeout <= 0 {
		cfg.Server.RequestTimeout = 10 * time.Second
	}
	if cfg.App.MarkdownRendererPool <= 0 {
		cfg.App.MarkdownRendererPool = 4
	}
	if cfg.App.SettingsCacheTTL <= 0 {
		cfg.App.SettingsCacheTTL = 30 * time.Second
	}
	if cfg.App.TagCloudCacheTTL <= 0 {
		cfg.App.TagCloudCacheTTL = 30 * time.Second
	}

	// DB sanity.
	if cfg.DB.MaxOpenConns <= 0 {
		cfg.DB.MaxOpenConns = 50
	}
	if cfg.DB.MaxIdleConns < 0 {
		cfg.DB.MaxIdleConns = 0
	}
	if cfg.DB.MaxIdleConns > cfg.DB.MaxOpenConns {
		cfg.DB.MaxIdleConns = cfg.DB.MaxOpenConns
	}
	if cfg.DB.ConnMaxLifetime <= 0 {
		cfg.DB.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.DB.ConnMaxIdleTime <= 0 {
		cfg.DB.ConnMaxIdleTime = 5 * time.Minute
	}
	if cfg.DB.PingTimeout <= 0 {
		cfg.DB.PingTimeout = 5 * time.Second
	}

	return cfg, nil
}

func Load() (Config, error) {
	var cfgPath string
	flag.StringVar(&cfgPath, "config", "", "path to YAML config file (optional)")
	flag.Parse()

	return LoadFromPath(cfgPath)
}

func applyEnv(cfg *Config) {
	parseDur := func(key string, dst *time.Duration) {
		if v := os.Getenv(key); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				*dst = d
			}
		}
	}
	parseInt := func(key string, dst *int) {
		if v := os.Getenv(key); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				*dst = n
			}
		}
	}

	// Server
	if v := os.Getenv("BLOGCMS_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	parseDur("BLOGCMS_READ_HEADER_TIMEOUT", &cfg.Server.ReadHeaderTimeout)
	parseDur("BLOGCMS_READ_TIMEOUT", &cfg.Server.ReadTimeout)
	parseDur("BLOGCMS_WRITE_TIMEOUT", &cfg.Server.WriteTimeout)
	parseDur("BLOGCMS_IDLE_TIMEOUT", &cfg.Server.IdleTimeout)
	parseDur("BLOGCMS_SHUTDOWN_TIMEOUT", &cfg.Server.ShutdownTimeout)
	parseDur("BLOGCMS_REQUEST_TIMEOUT", &cfg.Server.RequestTimeout)
	parseInt("BLOGCMS_MAX_HEADER_BYTES", &cfg.Server.MaxHeaderBytes)

	// DB
	if v := os.Getenv("BLOGCMS_DSN"); v != "" {
		cfg.DB.DSN = v
	}
	parseInt("BLOGCMS_DB_MAX_OPEN_CONNS", &cfg.DB.MaxOpenConns)
	parseInt("BLOGCMS_DB_MAX_IDLE_CONNS", &cfg.DB.MaxIdleConns)
	parseDur("BLOGCMS_DB_CONN_MAX_LIFETIME", &cfg.DB.ConnMaxLifetime)
	parseDur("BLOGCMS_DB_CONN_MAX_IDLE_TIME", &cfg.DB.ConnMaxIdleTime)
	parseDur("BLOGCMS_DB_PING_TIMEOUT", &cfg.DB.PingTimeout)
	// Security
	if v := os.Getenv("BLOGCMS_SESSION_KEY"); v != "" {
		cfg.Security.SessionKey = v
	}
	if v := os.Getenv("BLOGCMS_COOKIE_SECURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.Security.CookieSecure = b
		}
	}
	// Storage
	if v := os.Getenv("BLOGCMS_UPLOAD_DIR"); v != "" {
		cfg.Storage.UploadDir = v
	}
	if v := os.Getenv("BLOGCMS_UPLOAD_PUBLIC_BASE"); v != "" {
		cfg.Storage.PublicBaseURL = v
	}
	if v := os.Getenv("BLOGCMS_MAX_UPLOAD_MB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Storage.MaxUploadMB = n
		}
	}

	// App
	parseInt("BLOGCMS_MD_POOL", &cfg.App.MarkdownRendererPool)
	parseDur("BLOGCMS_SETTINGS_CACHE_TTL", &cfg.App.SettingsCacheTTL)
	parseDur("BLOGCMS_TAGCLOUD_CACHE_TTL", &cfg.App.TagCloudCacheTTL)
}
