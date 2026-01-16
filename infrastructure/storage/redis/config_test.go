package redis

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	if cfg.Address != "localhost:6379" {
		t.Errorf("Address = %s, want localhost:6379", cfg.Address)
	}
	if cfg.Password != "" {
		t.Errorf("Password = %s, want empty", cfg.Password)
	}
	if cfg.DB != 0 {
		t.Errorf("DB = %d, want 0", cfg.DB)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", cfg.MaxRetries)
	}
	if cfg.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, 5*time.Second)
	}
	if cfg.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, 3*time.Second)
	}
	if cfg.WriteTimeout != 3*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, 3*time.Second)
	}
	if cfg.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want 10", cfg.PoolSize)
	}
	if cfg.MinIdleConns != 2 {
		t.Errorf("MinIdleConns = %d, want 2", cfg.MinIdleConns)
	}
	if cfg.KeyPrefix != "agent:" {
		t.Errorf("KeyPrefix = %s, want agent:", cfg.KeyPrefix)
	}
}

func TestWithAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		address  string
		expected string
	}{
		{"localhost", "localhost:6379", "localhost:6379"},
		{"custom host", "redis.example.com:6380", "redis.example.com:6380"},
		{"ip address", "192.168.1.100:6379", "192.168.1.100:6379"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithAddress(tt.address)(&cfg)

			if cfg.Address != tt.expected {
				t.Errorf("Address = %s, want %s", cfg.Address, tt.expected)
			}
		})
	}
}

func TestWithPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
	}{
		{"simple password", "secret123"},
		{"empty password", ""},
		{"complex password", "p@ss=w0rd!#$%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithPassword(tt.password)(&cfg)

			if cfg.Password != tt.password {
				t.Errorf("Password = %s, want %s", cfg.Password, tt.password)
			}
		})
	}
}

func TestWithDB(t *testing.T) {
	t.Parallel()

	tests := []int{0, 1, 5, 15}

	for _, db := range tests {
		t.Run("db_"+string(rune('0'+db)), func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithDB(db)(&cfg)

			if cfg.DB != db {
				t.Errorf("DB = %d, want %d", cfg.DB, db)
			}
		})
	}
}

func TestWithKeyPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		prefix   string
		expected string
	}{
		{"default prefix", "agent:", "agent:"},
		{"custom prefix", "myapp:", "myapp:"},
		{"empty prefix", "", ""},
		{"nested prefix", "prod:service:cache:", "prod:service:cache:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithKeyPrefix(tt.prefix)(&cfg)

			if cfg.KeyPrefix != tt.expected {
				t.Errorf("KeyPrefix = %s, want %s", cfg.KeyPrefix, tt.expected)
			}
		})
	}
}

func TestWithPoolSize(t *testing.T) {
	t.Parallel()

	tests := []int{1, 5, 10, 50, 100}

	for _, size := range tests {
		t.Run("size_"+string(rune('0'+size%10)), func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithPoolSize(size)(&cfg)

			if cfg.PoolSize != size {
				t.Errorf("PoolSize = %d, want %d", cfg.PoolSize, size)
			}
		})
	}
}

func TestWithTimeouts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		dial         time.Duration
		read         time.Duration
		write        time.Duration
	}{
		{
			name:  "default timeouts",
			dial:  5 * time.Second,
			read:  3 * time.Second,
			write: 3 * time.Second,
		},
		{
			name:  "fast timeouts",
			dial:  time.Second,
			read:  500 * time.Millisecond,
			write: 500 * time.Millisecond,
		},
		{
			name:  "slow timeouts",
			dial:  30 * time.Second,
			read:  10 * time.Second,
			write: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultConfig()
			WithTimeouts(tt.dial, tt.read, tt.write)(&cfg)

			if cfg.DialTimeout != tt.dial {
				t.Errorf("DialTimeout = %v, want %v", cfg.DialTimeout, tt.dial)
			}
			if cfg.ReadTimeout != tt.read {
				t.Errorf("ReadTimeout = %v, want %v", cfg.ReadTimeout, tt.read)
			}
			if cfg.WriteTimeout != tt.write {
				t.Errorf("WriteTimeout = %v, want %v", cfg.WriteTimeout, tt.write)
			}
		})
	}
}

func TestConfigOptions_Chaining(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	opts := []ConfigOption{
		WithAddress("redis.prod.example.com:6380"),
		WithPassword("production-secret"),
		WithDB(3),
		WithKeyPrefix("prod:myapp:"),
		WithPoolSize(25),
		WithTimeouts(10*time.Second, 5*time.Second, 5*time.Second),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	if cfg.Address != "redis.prod.example.com:6380" {
		t.Errorf("Address = %s, want redis.prod.example.com:6380", cfg.Address)
	}
	if cfg.Password != "production-secret" {
		t.Errorf("Password = %s, want production-secret", cfg.Password)
	}
	if cfg.DB != 3 {
		t.Errorf("DB = %d, want 3", cfg.DB)
	}
	if cfg.KeyPrefix != "prod:myapp:" {
		t.Errorf("KeyPrefix = %s, want prod:myapp:", cfg.KeyPrefix)
	}
	if cfg.PoolSize != 25 {
		t.Errorf("PoolSize = %d, want 25", cfg.PoolSize)
	}
	if cfg.DialTimeout != 10*time.Second {
		t.Errorf("DialTimeout = %v, want 10s", cfg.DialTimeout)
	}
	if cfg.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want 5s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 5*time.Second {
		t.Errorf("WriteTimeout = %v, want 5s", cfg.WriteTimeout)
	}
}

func TestConfigOption_OverrideOrder(t *testing.T) {
	t.Parallel()

	cfg := DefaultConfig()

	// Apply options that override each other
	opts := []ConfigOption{
		WithAddress("first.example.com:6379"),
		WithAddress("second.example.com:6379"),
		WithAddress("final.example.com:6379"),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	// Last option should win
	if cfg.Address != "final.example.com:6379" {
		t.Errorf("Address = %s, want final.example.com:6379", cfg.Address)
	}
}
