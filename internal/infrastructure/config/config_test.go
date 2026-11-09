package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("DEBUG_AUTH_STUB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.AllowDebugAuthStub() {
		t.Fatal("debug auth stub must not be enabled by default")
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9090")
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("APP_ENV", "")
	t.Setenv("DEBUG_AUTH_STUB", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9090" {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, "127.0.0.1:9090")
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadValidatesteFailFast(t *testing.T) {
	tests := []struct {
		name     string
		httpAddr string
		logLevel string
	}{
		{name: "invalid log level", httpAddr: ":8080", logLevel: "trace"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HTTP_ADDR", tt.httpAddr)
			t.Setenv("LOG_LEVEL", tt.logLevel)

			_, err := Load()
			if err == nil {
				t.Fatal("Load() error = nil, want validatestion error")
			}
		})
	}
}

func TestValidateste(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{name: "valid", cfg: Config{HTTPAddr: ":8080", LogLevel: "info"}, wantErr: false},
		{name: "valid debug", cfg: Config{HTTPAddr: ":8080", LogLevel: "debug"}, wantErr: false},
		{name: "valid warn", cfg: Config{HTTPAddr: ":8080", LogLevel: "warn"}, wantErr: false},
		{name: "valid error", cfg: Config{HTTPAddr: ":8080", LogLevel: "error"}, wantErr: false},
		{name: "missing addr", cfg: Config{HTTPAddr: "", LogLevel: "info"}, wantErr: true},
		{name: "invalid level", cfg: Config{HTTPAddr: ":8080", LogLevel: "verbose"}, wantErr: true},
		{name: "debug stub allowed in dev", cfg: Config{HTTPAddr: ":8080", LogLevel: "info", AppEnv: "dev", DebugAuthStub: true}, wantErr: false},
		{name: "debug stub rejected outside dev", cfg: Config{HTTPAddr: ":8080", LogLevel: "info", AppEnv: "prod", DebugAuthStub: true}, wantErr: true},
		{name: "debug stub rejected with empty env", cfg: Config{HTTPAddr: ":8080", LogLevel: "info", DebugAuthStub: true}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validateste()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validateste() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
