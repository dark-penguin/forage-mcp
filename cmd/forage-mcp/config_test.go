package main

import (
	"log/slog"
	"os"
	"testing"
)


func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		args         []string
		wantErr      bool
		wantURL      string
		wantKey      string
		wantTimeout  int
		wantLogLevel slog.Level
		checkLevel   bool
	}{
		{
			name:        "env only",
			env:         map[string]string{"FORAGE_URL": "http://env-host:3672", "FORAGE_API_KEY": "env-key"},
			args:        []string{},
			wantErr:     false,
			wantURL:     "http://env-host:3672",
			wantKey:     "env-key",
			wantTimeout: 120,
		},
		{
			name:        "flag overrides env",
			env:         map[string]string{"FORAGE_URL": "http://env-host:3672", "FORAGE_API_KEY": "env-key"},
			args:        []string{"-u", "http://flag-host:3672", "-k", "flag-key"},
			wantErr:     false,
			wantURL:     "http://flag-host:3672",
			wantKey:     "flag-key",
			wantTimeout: 120,
		},
		{
			name:      "missing URL returns error",
			env:       map[string]string{},
			args:      []string{},
			wantErr:   true,
		},
		{
			name:        "timeout default is 120",
			env:         map[string]string{"FORAGE_URL": "http://localhost:3672"},
			args:        []string{},
			wantErr:     false,
			wantTimeout: 120,
		},
		{
			name:        "timeout flag override",
			env:         map[string]string{"FORAGE_URL": "http://localhost:3672"},
			args:        []string{"-t", "60"},
			wantErr:     false,
			wantTimeout: 60,
		},
		{
			name:         "verbose sets debug level",
			env:          map[string]string{"FORAGE_URL": "http://localhost:3672"},
			args:         []string{"-v"},
			wantErr:      false,
			wantLogLevel: slog.LevelDebug,
			checkLevel:   true,
		},
		{
			name:         "quiet sets warn level",
			env:          map[string]string{"FORAGE_URL": "http://localhost:3672"},
			args:         []string{"-q"},
			wantErr:      false,
			wantLogLevel: slog.LevelWarn,
			checkLevel:   true,
		},
		{
			name:         "LOG_LEVEL env sets debug",
			env:          map[string]string{"FORAGE_URL": "http://localhost:3672", "LOG_LEVEL": "debug"},
			args:         []string{},
			wantErr:      false,
			wantLogLevel: slog.LevelDebug,
			checkLevel:   true,
		},
		{
			name:         "verbose flag overrides LOG_LEVEL",
			env:          map[string]string{"FORAGE_URL": "http://localhost:3672", "LOG_LEVEL": "warn"},
			args:         []string{"-v"},
			wantErr:      false,
			wantLogLevel: slog.LevelDebug,
			checkLevel:   true,
		},
		{
			name:    "timeout 0 returns error",
			env:     map[string]string{"FORAGE_URL": "http://localhost:3672"},
			args:    []string{"-t", "0"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Override os.Args for pflag parsing
			origArgs := os.Args
			os.Args = append([]string{"forage-mcp"}, tt.args...)
			t.Cleanup(func() { os.Args = origArgs })

			// Save and restore env vars via t.Cleanup for panic safety
			for k, v := range tt.env {
				if old, ok := os.LookupEnv(k); ok {
					t.Cleanup(func() { os.Setenv(k, old) })
				} else {
					t.Cleanup(func() { os.Unsetenv(k) })
				}
				os.Setenv(k, v)
			}
			// Clear env vars not in the test
			for _, key := range []string{"FORAGE_URL", "FORAGE_API_KEY", "LOG_LEVEL"} {
				if _, ok := tt.env[key]; !ok {
					if old, wasSet := os.LookupEnv(key); wasSet {
						t.Cleanup(func() { os.Setenv(key, old) })
					}
					os.Unsetenv(key)
				}
			}

			cfg, err := LoadConfig()

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantURL != "" && cfg.ForageURL != tt.wantURL {
				t.Errorf("ForageURL = %q, want %q", cfg.ForageURL, tt.wantURL)
			}
			if tt.wantKey != "" && cfg.APIKey != tt.wantKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.wantKey)
			}
			if tt.wantTimeout > 0 && cfg.TimeoutSeconds != tt.wantTimeout {
				t.Errorf("TimeoutSeconds = %d, want %d", cfg.TimeoutSeconds, tt.wantTimeout)
			}
			if tt.checkLevel && cfg.LogLevel != tt.wantLogLevel {
				t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, tt.wantLogLevel)
			}
		})
	}
}
