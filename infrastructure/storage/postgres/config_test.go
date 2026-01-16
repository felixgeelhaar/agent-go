package postgres

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Host != "localhost" {
		t.Errorf("Host = %s, want localhost", cfg.Host)
	}
	if cfg.Port != 5432 {
		t.Errorf("Port = %d, want 5432", cfg.Port)
	}
	if cfg.Database != "agent" {
		t.Errorf("Database = %s, want agent", cfg.Database)
	}
	if cfg.User != "postgres" {
		t.Errorf("User = %s, want postgres", cfg.User)
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("SSLMode = %s, want disable", cfg.SSLMode)
	}
	if cfg.MaxConns != 10 {
		t.Errorf("MaxConns = %d, want 10", cfg.MaxConns)
	}
	if cfg.MinConns != 2 {
		t.Errorf("MinConns = %d, want 2", cfg.MinConns)
	}
	if cfg.MaxConnLifetime != time.Hour {
		t.Errorf("MaxConnLifetime = %v, want %v", cfg.MaxConnLifetime, time.Hour)
	}
	if cfg.MaxConnIdleTime != 30*time.Minute {
		t.Errorf("MaxConnIdleTime = %v, want %v", cfg.MaxConnIdleTime, 30*time.Minute)
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("ConnectTimeout = %v, want %v", cfg.ConnectTimeout, 10*time.Second)
	}
	if cfg.Schema != "public" {
		t.Errorf("Schema = %s, want public", cfg.Schema)
	}
}

func TestConfig_ConnectionString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name:     "default config",
			config:   DefaultConfig(),
			expected: "host=localhost port=5432 dbname=agent user=postgres password= sslmode=disable",
		},
		{
			name: "custom config",
			config: Config{
				Host:     "db.example.com",
				Port:     5433,
				Database: "myapp",
				User:     "appuser",
				Password: "secret123",
				SSLMode:  "require",
			},
			expected: "host=db.example.com port=5433 dbname=myapp user=appuser password=secret123 sslmode=require",
		},
		{
			name: "with special characters in password",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				Database: "test",
				User:     "user",
				Password: "p@ss=word",
				SSLMode:  "disable",
			},
			expected: "host=localhost port=5432 dbname=test user=user password=p@ss=word sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.config.ConnectionString()
			if result != tt.expected {
				t.Errorf("ConnectionString() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestWithHost(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithHost("custom.host.com")(&cfg)

	if cfg.Host != "custom.host.com" {
		t.Errorf("Host = %s, want custom.host.com", cfg.Host)
	}
}

func TestWithPort(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithPort(5433)(&cfg)

	if cfg.Port != 5433 {
		t.Errorf("Port = %d, want 5433", cfg.Port)
	}
}

func TestWithDatabase(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithDatabase("mydb")(&cfg)

	if cfg.Database != "mydb" {
		t.Errorf("Database = %s, want mydb", cfg.Database)
	}
}

func TestWithCredentials(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithCredentials("myuser", "mypassword")(&cfg)

	if cfg.User != "myuser" {
		t.Errorf("User = %s, want myuser", cfg.User)
	}
	if cfg.Password != "mypassword" {
		t.Errorf("Password = %s, want mypassword", cfg.Password)
	}
}

func TestWithSSLMode(t *testing.T) {
	t.Parallel()

	tests := []string{"disable", "require", "verify-ca", "verify-full"}

	for _, mode := range tests {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithSSLMode(mode)(&cfg)

			if cfg.SSLMode != mode {
				t.Errorf("SSLMode = %s, want %s", cfg.SSLMode, mode)
			}
		})
	}
}

func TestWithPoolSize(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithPoolSize(5, 20)(&cfg)

	if cfg.MinConns != 5 {
		t.Errorf("MinConns = %d, want 5", cfg.MinConns)
	}
	if cfg.MaxConns != 20 {
		t.Errorf("MaxConns = %d, want 20", cfg.MaxConns)
	}
}

func TestWithSchema(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()
	WithSchema("custom_schema")(&cfg)

	if cfg.Schema != "custom_schema" {
		t.Errorf("Schema = %s, want custom_schema", cfg.Schema)
	}
}

func TestConfigOptions_Chaining(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	opts := []ConfigOption{
		WithHost("db.example.com"),
		WithPort(5433),
		WithDatabase("production"),
		WithCredentials("admin", "adminpass"),
		WithSSLMode("verify-full"),
		WithPoolSize(10, 50),
		WithSchema("app_schema"),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.Host != "db.example.com" {
		t.Errorf("Host = %s, want db.example.com", cfg.Host)
	}
	if cfg.Port != 5433 {
		t.Errorf("Port = %d, want 5433", cfg.Port)
	}
	if cfg.Database != "production" {
		t.Errorf("Database = %s, want production", cfg.Database)
	}
	if cfg.User != "admin" {
		t.Errorf("User = %s, want admin", cfg.User)
	}
	if cfg.Password != "adminpass" {
		t.Errorf("Password = %s, want adminpass", cfg.Password)
	}
	if cfg.SSLMode != "verify-full" {
		t.Errorf("SSLMode = %s, want verify-full", cfg.SSLMode)
	}
	if cfg.MinConns != 10 {
		t.Errorf("MinConns = %d, want 10", cfg.MinConns)
	}
	if cfg.MaxConns != 50 {
		t.Errorf("MaxConns = %d, want 50", cfg.MaxConns)
	}
	if cfg.Schema != "app_schema" {
		t.Errorf("Schema = %s, want app_schema", cfg.Schema)
	}

	// Verify connection string is correct
	connStr := cfg.ConnectionString()
	expected := "host=db.example.com port=5433 dbname=production user=admin password=adminpass sslmode=verify-full"
	if connStr != expected {
		t.Errorf("ConnectionString() = %s, want %s", connStr, expected)
	}
}
