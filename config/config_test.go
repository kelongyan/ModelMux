package config

import (
	"testing"

	"github.com/kelongyan/ModelMux/state"
)

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
	if cfg.StreamKeepAliveSeconds != DefaultStreamKeepAliveSeconds {
		t.Fatalf("StreamKeepAliveSeconds = %d, want %d", cfg.StreamKeepAliveSeconds, DefaultStreamKeepAliveSeconds)
	}
	if cfg.StreamIdleTimeoutSeconds != DefaultStreamIdleTimeoutSeconds {
		t.Fatalf("StreamIdleTimeoutSeconds = %d, want %d", cfg.StreamIdleTimeoutSeconds, DefaultStreamIdleTimeoutSeconds)
	}
	if cfg.StreamMaxDurationSeconds != DefaultStreamMaxDurationSeconds {
		t.Fatalf("StreamMaxDurationSeconds = %d, want %d", cfg.StreamMaxDurationSeconds, DefaultStreamMaxDurationSeconds)
	}
	if cfg.MaxTransientRetries != DefaultMaxTransientRetries {
		t.Fatalf("MaxTransientRetries = %d, want %d", cfg.MaxTransientRetries, DefaultMaxTransientRetries)
	}
	if cfg.ProviderCircuitFailureThreshold != DefaultProviderCircuitFailureThreshold {
		t.Fatalf("ProviderCircuitFailureThreshold = %d, want %d", cfg.ProviderCircuitFailureThreshold, DefaultProviderCircuitFailureThreshold)
	}
	if cfg.ProviderCircuitOpenSeconds != DefaultProviderCircuitOpenSeconds {
		t.Fatalf("ProviderCircuitOpenSeconds = %d, want %d", cfg.ProviderCircuitOpenSeconds, DefaultProviderCircuitOpenSeconds)
	}
	if cfg.ProviderCircuitMaxOpenSeconds != DefaultProviderCircuitMaxOpenSeconds {
		t.Fatalf("ProviderCircuitMaxOpenSeconds = %d, want %d", cfg.ProviderCircuitMaxOpenSeconds, DefaultProviderCircuitMaxOpenSeconds)
	}
	if cfg.ProviderCircuitHalfOpenMax != DefaultProviderCircuitHalfOpenMax {
		t.Fatalf("ProviderCircuitHalfOpenMax = %d, want %d", cfg.ProviderCircuitHalfOpenMax, DefaultProviderCircuitHalfOpenMax)
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

func TestValidateRejectsProviderIDWithSlash(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{ID: "bad/id", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
		ActiveProvider: "bad/id",
	}

	if err := cfg.validate(); err == nil {
		t.Fatal("validate() error = nil, want invalid provider id error")
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

func TestProviderKeyMetadataHelpers(t *testing.T) {
	cfg := &Config{
		ActiveProvider: "p1",
		Providers: []ProviderConfig{{
			ID:        "p1",
			TargetURL: "https://one.example.com",
			Keys:      []string{"k1", "k2"},
			KeyMetadata: map[string]KeyMetadata{
				state.KeyID("k2"): KeyMetadata{Label: "备用", Note: "暂停轮询", Disabled: true},
				state.KeyID("old"): KeyMetadata{
					Label: "orphan",
				},
			},
		}},
	}

	cfg.applyDefaults()

	provider := cfg.Providers[0]
	if len(provider.KeyMetadata) != 1 {
		t.Fatalf("len(KeyMetadata) = %d, want 1 after pruning orphan metadata", len(provider.KeyMetadata))
	}
	enabled := provider.EnabledKeys()
	if len(enabled) != 1 || enabled[0] != "k1" {
		t.Fatalf("EnabledKeys() = %v, want [k1]", enabled)
	}
	if disabled := provider.DisabledKeyCount(); disabled != 1 {
		t.Fatalf("DisabledKeyCount() = %d, want 1", disabled)
	}
	meta, ok := provider.KeyMetadataForValue("k2")
	if !ok {
		t.Fatal("KeyMetadataForValue(k2) ok = false, want true")
	}
	if meta.Label != "备用" || meta.Note != "暂停轮询" || !meta.Disabled {
		t.Fatalf("metadata = %+v, want label/note/disabled", meta)
	}
}

func TestProviderConfigCopyDoesNotShareKeyMetadata(t *testing.T) {
	original := ProviderConfig{
		ID:        "p1",
		TargetURL: "https://one.example.com",
		Keys:      []string{"k1"},
		KeyMetadata: map[string]KeyMetadata{
			state.KeyID("k1"): KeyMetadata{Label: "主力"},
		},
	}

	copied := original.copy()
	copied.KeyMetadata[state.KeyID("k1")] = KeyMetadata{Label: "changed"}

	if original.KeyMetadata[state.KeyID("k1")].Label != "主力" {
		t.Fatalf("original metadata changed to %+v", original.KeyMetadata[state.KeyID("k1")])
	}
}

func TestValidateAcceptsValidProxyURLs(t *testing.T) {
	cfg := &Config{
		ProxyURL: "http://127.0.0.1:7897",
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}, ProxyURL: "socks5://127.0.0.1:7891"},
			{ID: "p2", TargetURL: "https://two.example.com", Keys: []string{"k2"}, ProxyURL: "direct"},
		},
		ActiveProvider: "p1",
	}

	if err := cfg.validate(); err != nil {
		t.Fatalf("validate() error = %v, want nil", err)
	}
}

func TestValidateRejectsInvalidProxyURLs(t *testing.T) {
	cases := []struct {
		name   string
		global string
	}{
		{name: "bad global scheme", global: "ftp://127.0.0.1:7897"},
		{name: "global without host", global: "http://"},
		{name: "relative proxy url", global: "/tmp/socks"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				ProxyURL:  tc.global,
				TargetURL: "https://example.com",
				Keys:      []string{"k1"},
			}
			if err := cfg.validate(); err == nil {
				t.Fatal("validate() error = nil, want invalid proxy_url error")
			}
		})
	}

	providerCfg := &Config{
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}, ProxyURL: "ftp://127.0.0.1:21"},
		},
		ActiveProvider: "p1",
	}
	if err := providerCfg.validate(); err == nil {
		t.Fatal("validate() error = nil, want invalid providers[0].proxy_url error")
	}
}

func TestEffectiveProxyURLPrecedence(t *testing.T) {
	cfg := &Config{
		ProxyURL: "http://127.0.0.1:7897",
		Providers: []ProviderConfig{
			{ID: "inherit", TargetURL: "https://a.example.com", Keys: []string{"k1"}},
			{ID: "direct", TargetURL: "https://b.example.com", Keys: []string{"k2"}, ProxyURL: "direct"},
			{ID: "dash", TargetURL: "https://c.example.com", Keys: []string{"k3"}, ProxyURL: "-"},
			{ID: "override", TargetURL: "https://d.example.com", Keys: []string{"k4"}, ProxyURL: "socks5h://127.0.0.1:7891"},
		},
	}

	providers := cfg.ProviderConfigs()
	if got := cfg.EffectiveProxyURL(providers[0]); got != "http://127.0.0.1:7897" {
		t.Fatalf("inherit proxy = %q, want global value", got)
	}
	if got := cfg.EffectiveProxyURL(providers[1]); got != "" {
		t.Fatalf("direct proxy = %q, want empty for direct connection", got)
	}
	if got := cfg.EffectiveProxyURL(providers[2]); got != "" {
		t.Fatalf("dash proxy = %q, want empty for direct connection", got)
	}
	if got := cfg.EffectiveProxyURL(providers[3]); got != "socks5h://127.0.0.1:7891" {
		t.Fatalf("override proxy = %q, want socks5h override", got)
	}
}

func TestEffectiveProxyURLWithoutGlobal(t *testing.T) {
	cfg := &Config{
		Providers: []ProviderConfig{
			{ID: "p1", TargetURL: "https://one.example.com", Keys: []string{"k1"}},
		},
	}

	if got := cfg.EffectiveProxyURL(cfg.ProviderConfigs()[0]); got != "" {
		t.Fatalf("proxy = %q, want empty when nothing configured", got)
	}
}
