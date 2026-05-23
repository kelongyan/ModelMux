package config

import "testing"

func TestApplyDefaultsUsesSafeLocalAdminAndBodyLimit(t *testing.T) {
	cfg := &Config{
		TargetURL: "https://example.com",
		Keys:      []string{"k1"},
	}

	cfg.applyDefaults()

	if cfg.Listen != ":8080" {
		t.Fatalf("Listen = %q, want :8080", cfg.Listen)
	}
	if cfg.AdminListen != "127.0.0.1:8081" {
		t.Fatalf("AdminListen = %q, want 127.0.0.1:8081", cfg.AdminListen)
	}
	if cfg.ActiveProvider != DefaultProviderID {
		t.Fatalf("ActiveProvider = %q, want %q", cfg.ActiveProvider, DefaultProviderID)
	}
	if len(cfg.Providers) != 1 {
		t.Fatalf("len(Providers) = %d, want 1", len(cfg.Providers))
	}
	if cfg.Providers[0].ID != DefaultProviderID {
		t.Fatalf("Providers[0].ID = %q, want %q", cfg.Providers[0].ID, DefaultProviderID)
	}
	if cfg.MaxBodyBytes != DefaultMaxBodyBytes {
		t.Fatalf("MaxBodyBytes = %d, want %d", cfg.MaxBodyBytes, DefaultMaxBodyBytes)
	}
	if cfg.LogOutput != DefaultLogOutput {
		t.Fatalf("LogOutput = %q, want %q", cfg.LogOutput, DefaultLogOutput)
	}
	if cfg.LogMaxSizeMB != DefaultLogMaxSizeMB {
		t.Fatalf("LogMaxSizeMB = %d, want %d", cfg.LogMaxSizeMB, DefaultLogMaxSizeMB)
	}
	if cfg.LogMaxBackups != DefaultLogMaxBackups {
		t.Fatalf("LogMaxBackups = %d, want %d", cfg.LogMaxBackups, DefaultLogMaxBackups)
	}
	if cfg.LogMaxAgeDays != DefaultLogMaxAgeDays {
		t.Fatalf("LogMaxAgeDays = %d, want %d", cfg.LogMaxAgeDays, DefaultLogMaxAgeDays)
	}
	if cfg.StateFile != DefaultStateFile {
		t.Fatalf("StateFile = %q, want %q", cfg.StateFile, DefaultStateFile)
	}
	if cfg.InvalidTTLHours != DefaultInvalidTTLHours {
		t.Fatalf("InvalidTTLHours = %d, want %d", cfg.InvalidTTLHours, DefaultInvalidTTLHours)
	}
	if !cfg.StatePersistenceEnabled() {
		t.Fatal("StatePersistenceEnabled() = false, want true by default")
	}
}

func TestValidateUsesFirstProviderWhenActiveMissing(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestApplyDefaultsUsesFirstProviderWhenActiveMissing(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
	}

	cfg.applyDefaults()

	if cfg.ActiveProvider != "p1" {
		t.Fatalf("ActiveProvider = %q, want p1", cfg.ActiveProvider)
	}
	if cfg.TargetURL != "https://one.example.com" {
		t.Fatalf("TargetURL = %q, want https://one.example.com", cfg.TargetURL)
	}
	if len(cfg.Keys) != 1 || cfg.Keys[0] != "k1" {
		t.Fatalf("Keys = %v, want [k1]", cfg.Keys)
	}
}

func TestValidateRejectsRelativeTargetURL(t *testing.T) {
	cfg := &Config{
		TargetURL: "/relative",
		Keys:      []string{"k1"},
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("validate() error = nil, want relative target_url error")
	}
}

func TestApplyDefaultsUsesBothOutputWhenLogFileConfigured(t *testing.T) {
	cfg := &Config{
		TargetURL: "https://example.com",
		Keys:      []string{"k1"},
		LogFile:   "logs/proxy.log",
	}

	cfg.applyDefaults()

	if cfg.LogOutput != "both" {
		t.Fatalf("LogOutput = %q, want both", cfg.LogOutput)
	}
}

func TestValidateAfterDefaultsRejectsInvalidLogOutput(t *testing.T) {
	cfg := &Config{LogOutput: "invalid"}

	if err := cfg.validateAfterDefaults(); err == nil {
		t.Fatal("validateAfterDefaults() error = nil, want invalid log_output error")
	}
}

func TestValidateAfterDefaultsRequiresLogFileForFileOutput(t *testing.T) {
	cfg := &Config{LogOutput: "file"}

	if err := cfg.validateAfterDefaults(); err == nil {
		t.Fatal("validateAfterDefaults() error = nil, want missing log_file error")
	}
}

func TestValidateRejectsDuplicateProviderIDs(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
			{ID: "p1", TargetURL: "https://two.example.com", Keys: []string{"k2"}},
		},
		ActiveProvider: "p1",
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("validate() error = nil, want duplicate provider id error")
	}
}

func TestValidateLegacyConfigStillWorks(t *testing.T) {
	cfg := &Config{
		TargetURL: "https://example.com",
		Keys:      []string{"k1"},
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() error = %v", err)
	}
}

func TestStatePersistenceCanBeDisabled(t *testing.T) {
	disabled := false
	cfg := &Config{PersistState: &disabled}

	if cfg.StatePersistenceEnabled() {
		t.Fatal("StatePersistenceEnabled() = true, want false")
	}
}
