package config

import (
	"encoding/json"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_JSON_MarshalUnmarshal_Roundtrip(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		wantJSON string
	}{
		{
			name:     "zero value",
			duration: Duration(0),
			wantJSON: `"0s"`,
		},
		{
			name:     "5 seconds",
			duration: Duration(5 * time.Second),
			wantJSON: `"5s"`,
		},
		{
			name:     "1 minute 30 seconds",
			duration: Duration(90 * time.Second),
			wantJSON: `"1m30s"`,
		},
		{
			name:     "2 hours",
			duration: Duration(2 * time.Hour),
			wantJSON: `"2h0m0s"`,
		},
		{
			name:     "complex duration",
			duration: Duration(2*time.Hour + 30*time.Minute + 45*time.Second),
			wantJSON: `"2h30m45s"`,
		},
		{
			name:     "milliseconds",
			duration: Duration(500 * time.Millisecond),
			wantJSON: `"500ms"`,
		},
		{
			name:     "microseconds",
			duration: Duration(250 * time.Microsecond),
			wantJSON: `"250µs"`,
		},
		{
			name:     "nanoseconds",
			duration: Duration(100 * time.Nanosecond),
			wantJSON: `"100ns"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			gotJSON, err := json.Marshal(tt.duration)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(gotJSON) != tt.wantJSON {
				t.Errorf("Marshal() = %s, want %s", string(gotJSON), tt.wantJSON)
			}

			// Unmarshal back
			var got Duration
			if err := json.Unmarshal(gotJSON, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got != tt.duration {
				t.Errorf("Roundtrip failed: got %v, want %v", got, tt.duration)
			}
		})
	}
}

func TestDuration_JSON_UnmarshalValidStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     Duration
		wantTime time.Duration
	}{
		{
			name:     "quoted 5s",
			input:    `"5s"`,
			want:     Duration(5 * time.Second),
			wantTime: 5 * time.Second,
		},
		{
			name:     "quoted 1m30s",
			input:    `"1m30s"`,
			want:     Duration(90 * time.Second),
			wantTime: 90 * time.Second,
		},
		{
			name:     "quoted 2h",
			input:    `"2h"`,
			want:     Duration(2 * time.Hour),
			wantTime: 2 * time.Hour,
		},
		{
			name:     "quoted 0s",
			input:    `"0s"`,
			want:     Duration(0),
			wantTime: 0,
		},
		{
			name:     "quoted complex",
			input:    `"3h45m12s"`,
			want:     Duration(3*time.Hour + 45*time.Minute + 12*time.Second),
			wantTime: 3*time.Hour + 45*time.Minute + 12*time.Second,
		},
		{
			name:     "quoted milliseconds",
			input:    `"100ms"`,
			want:     Duration(100 * time.Millisecond),
			wantTime: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration
			if err := json.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Unmarshal() = %v, want %v", got, tt.want)
			}

			if got.Duration() != tt.wantTime {
				t.Errorf("Duration() = %v, want %v", got.Duration(), tt.wantTime)
			}
		})
	}
}

func TestDuration_JSON_UnmarshalInvalidStrings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid format",
			input:   `"invalid"`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   `""`,
			wantErr: true,
		},
		{
			name:    "missing unit",
			input:   `"123"`,
			wantErr: true,
		},
		{
			name:    "invalid unit",
			input:   `"5x"`,
			wantErr: true,
		},
		{
			name:    "non-string numeric",
			input:   `123`,
			wantErr: true,
		},
		{
			name:    "negative duration",
			input:   `"-5s"`,
			wantErr: false, // time.ParseDuration allows negative durations
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr && err == nil {
				t.Errorf("Unmarshal() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unmarshal() unexpected error = %v", err)
			}
		})
	}
}

func TestDuration_JSON_UnmarshalNull(t *testing.T) {
	var d Duration
	err := json.Unmarshal([]byte("null"), &d)
	if err != nil {
		t.Errorf("Unmarshal(null) error = %v, want nil", err)
	}

	if d != Duration(0) {
		t.Errorf("Unmarshal(null) = %v, want 0", d)
	}
}

func TestDuration_YAML_MarshalUnmarshal_Roundtrip(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		wantYAML string
	}{
		{
			name:     "zero value",
			duration: Duration(0),
			wantYAML: "0s\n",
		},
		{
			name:     "5 seconds",
			duration: Duration(5 * time.Second),
			wantYAML: "5s\n",
		},
		{
			name:     "1 minute 30 seconds",
			duration: Duration(90 * time.Second),
			wantYAML: "1m30s\n",
		},
		{
			name:     "2 hours",
			duration: Duration(2 * time.Hour),
			wantYAML: "2h0m0s\n",
		},
		{
			name:     "complex duration",
			duration: Duration(2*time.Hour + 30*time.Minute + 45*time.Second),
			wantYAML: "2h30m45s\n",
		},
		{
			name:     "milliseconds",
			duration: Duration(750 * time.Millisecond),
			wantYAML: "750ms\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal
			gotYAML, err := yaml.Marshal(tt.duration)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			if string(gotYAML) != tt.wantYAML {
				t.Errorf("Marshal() = %q, want %q", string(gotYAML), tt.wantYAML)
			}

			// Unmarshal back
			var got Duration
			if err := yaml.Unmarshal(gotYAML, &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got != tt.duration {
				t.Errorf("Roundtrip failed: got %v, want %v", got, tt.duration)
			}
		})
	}
}

func TestDuration_YAML_UnmarshalValidStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     Duration
		wantTime time.Duration
	}{
		{
			name:     "5s",
			input:    "5s",
			want:     Duration(5 * time.Second),
			wantTime: 5 * time.Second,
		},
		{
			name:     "1m30s",
			input:    "1m30s",
			want:     Duration(90 * time.Second),
			wantTime: 90 * time.Second,
		},
		{
			name:     "2h",
			input:    "2h",
			want:     Duration(2 * time.Hour),
			wantTime: 2 * time.Hour,
		},
		{
			name:     "0s",
			input:    "0s",
			want:     Duration(0),
			wantTime: 0,
		},
		{
			name:     "complex",
			input:    "3h45m12s",
			want:     Duration(3*time.Hour + 45*time.Minute + 12*time.Second),
			wantTime: 3*time.Hour + 45*time.Minute + 12*time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration
			if err := yaml.Unmarshal([]byte(tt.input), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got != tt.want {
				t.Errorf("Unmarshal() = %v, want %v", got, tt.want)
			}

			if got.Duration() != tt.wantTime {
				t.Errorf("Duration() = %v, want %v", got.Duration(), tt.wantTime)
			}
		})
	}
}

func TestDuration_YAML_UnmarshalInvalidStrings(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: false, // YAML treats empty string as zero value, doesn't call UnmarshalYAML
		},
		{
			name:    "missing unit",
			input:   "123",
			wantErr: true,
		},
		{
			name:    "invalid unit",
			input:   "5x",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Duration
			err := yaml.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr && err == nil {
				t.Errorf("Unmarshal() expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unmarshal() unexpected error = %v", err)
			}
		})
	}
}

func TestDuration_InStruct_JSON(t *testing.T) {
	type Config struct {
		Timeout Duration `json:"timeout"`
		Delay   Duration `json:"delay,omitempty"`
	}

	tests := []struct {
		name    string
		input   string
		want    Config
		wantErr bool
	}{
		{
			name:  "valid config",
			input: `{"timeout":"30s","delay":"5s"}`,
			want: Config{
				Timeout: Duration(30 * time.Second),
				Delay:   Duration(5 * time.Second),
			},
			wantErr: false,
		},
		{
			name:  "omitempty field",
			input: `{"timeout":"1m"}`,
			want: Config{
				Timeout: Duration(60 * time.Second),
				Delay:   Duration(0),
			},
			wantErr: false,
		},
		{
			name:  "zero values",
			input: `{"timeout":"0s"}`,
			want: Config{
				Timeout: Duration(0),
				Delay:   Duration(0),
			},
			wantErr: false,
		},
		{
			name:    "invalid timeout",
			input:   `{"timeout":"invalid"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Config
			err := json.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Unmarshal() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
			if got.Delay != tt.want.Delay {
				t.Errorf("Delay = %v, want %v", got.Delay, tt.want.Delay)
			}

			// Marshal back and verify roundtrip
			data, err := json.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var roundtrip Config
			if err := json.Unmarshal(data, &roundtrip); err != nil {
				t.Fatalf("Roundtrip Unmarshal() error = %v", err)
			}

			if roundtrip.Timeout != got.Timeout {
				t.Errorf("Roundtrip Timeout = %v, want %v", roundtrip.Timeout, got.Timeout)
			}
		})
	}
}

func TestDuration_InStruct_YAML(t *testing.T) {
	type Config struct {
		Timeout Duration `yaml:"timeout"`
		Delay   Duration `yaml:"delay,omitempty"`
	}

	tests := []struct {
		name    string
		input   string
		want    Config
		wantErr bool
	}{
		{
			name: "valid config",
			input: `timeout: 30s
delay: 5s`,
			want: Config{
				Timeout: Duration(30 * time.Second),
				Delay:   Duration(5 * time.Second),
			},
			wantErr: false,
		},
		{
			name:  "omitempty field",
			input: `timeout: 1m`,
			want: Config{
				Timeout: Duration(60 * time.Second),
				Delay:   Duration(0),
			},
			wantErr: false,
		},
		{
			name:  "zero values",
			input: `timeout: 0s`,
			want: Config{
				Timeout: Duration(0),
				Delay:   Duration(0),
			},
			wantErr: false,
		},
		{
			name:    "invalid timeout",
			input:   `timeout: invalid`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Config
			err := yaml.Unmarshal([]byte(tt.input), &got)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Unmarshal() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
			if got.Delay != tt.want.Delay {
				t.Errorf("Delay = %v, want %v", got.Delay, tt.want.Delay)
			}

			// Marshal back and verify roundtrip
			data, err := yaml.Marshal(got)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var roundtrip Config
			if err := yaml.Unmarshal(data, &roundtrip); err != nil {
				t.Fatalf("Roundtrip Unmarshal() error = %v", err)
			}

			if roundtrip.Timeout != got.Timeout {
				t.Errorf("Roundtrip Timeout = %v, want %v", roundtrip.Timeout, got.Timeout)
			}
		})
	}
}

func TestDuration_Duration_Method(t *testing.T) {
	tests := []struct {
		name     string
		duration Duration
		want     time.Duration
	}{
		{
			name:     "zero",
			duration: Duration(0),
			want:     0,
		},
		{
			name:     "positive",
			duration: Duration(5 * time.Second),
			want:     5 * time.Second,
		},
		{
			name:     "large value",
			duration: Duration(24 * time.Hour),
			want:     24 * time.Hour,
		},
		{
			name:     "negative",
			duration: Duration(-10 * time.Second),
			want:     -10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.duration.Duration()
			if got != tt.want {
				t.Errorf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}
