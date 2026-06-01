package config

import "testing"

func TestApplyDefaultsUsesSafeLocalAdminAndBodyLimit(t *testing.T) {
	cfg := &Config{
		TargetURL: "https://example.com",
		Keys:      []string{"k1"},
	}

	cfg.applyDefaults()

	if cfg.Listen != DefaultListen {
		t.Fatalf("Listen = %q, want %q", cfg.Listen, DefaultListen)
	}
	if cfg.AdminListen != DefaultAdminListen {
		t.Fatalf("AdminListen = %q, want %q", cfg.AdminListen, DefaultAdminListen)
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
	if cfg.StatsDir != DefaultStatsDir {
		t.Fatalf("StatsDir = %q, want %q", cfg.StatsDir, DefaultStatsDir)
	}
	if cfg.StatsRetentionDays != DefaultStatsRetentionDays {
		t.Fatalf("StatsRetentionDays = %d, want %d", cfg.StatsRetentionDays, DefaultStatsRetentionDays)
	}
	if cfg.StatsMaxRecentRecords != DefaultStatsMaxRecentRecords {
		t.Fatalf("StatsMaxRecentRecords = %d, want %d", cfg.StatsMaxRecentRecords, DefaultStatsMaxRecentRecords)
	}
	if !cfg.StatsCollectionEnabled() {
		t.Fatal("StatsCollectionEnabled() = false, want true by default")
	}
	if cfg.ConnectTimeoutSeconds != DefaultConnectTimeoutSeconds {
		t.Fatalf("ConnectTimeoutSeconds = %d, want %d", cfg.ConnectTimeoutSeconds, DefaultConnectTimeoutSeconds)
	}
	if cfg.ResponseHeaderTimeoutSeconds != DefaultResponseHeaderTimeoutSeconds {
		t.Fatalf("ResponseHeaderTimeoutSeconds = %d, want %d", cfg.ResponseHeaderTimeoutSeconds, DefaultResponseHeaderTimeoutSeconds)
	}
	if cfg.TransientCoolingSeconds != DefaultTransientCoolingSeconds {
		t.Fatalf("TransientCoolingSeconds = %d, want %d", cfg.TransientCoolingSeconds, DefaultTransientCoolingSeconds)
	}
	if cfg.WaitForKeyTimeoutMS != DefaultWaitForKeyTimeoutMS {
		t.Fatalf("WaitForKeyTimeoutMS = %d, want %d", cfg.WaitForKeyTimeoutMS, DefaultWaitForKeyTimeoutMS)
	}
	if cfg.MaxTransientRetries != DefaultMaxTransientRetries {
		t.Fatalf("MaxTransientRetries = %d, want %d", cfg.MaxTransientRetries, DefaultMaxTransientRetries)
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

func TestStatsCollectionCanBeDisabled(t *testing.T) {
	disabled := false
	cfg := &Config{StatsEnabled: &disabled}

	if cfg.StatsCollectionEnabled() {
		t.Fatal("StatsCollectionEnabled() = true, want false")
	}
}
