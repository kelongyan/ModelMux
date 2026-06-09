package config

import "github.com/kelongyan/ModelMux/state"

func copyKeyMetadata(in map[string]KeyMetadata) map[string]KeyMetadata {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]KeyMetadata, len(in))
	for keyID, metadata := range in {
		if keyMetadataEmpty(metadata) {
			continue
		}
		out[keyID] = metadata
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func keyMetadataEmpty(metadata KeyMetadata) bool {
	return metadata.Label == "" && metadata.Note == "" && !metadata.Disabled
}

func (c *Config) normalizeKeyMetadata() {
	for i := range c.Providers {
		c.Providers[i].normalizeKeyMetadata()
	}
}

func (p *ProviderConfig) normalizeKeyMetadata() {
	if len(p.KeyMetadata) == 0 {
		p.KeyMetadata = nil
		return
	}

	keepIDs := make(map[string]struct{}, len(p.Keys))
	for _, key := range p.Keys {
		keepIDs[state.KeyID(key)] = struct{}{}
	}

	next := make(map[string]KeyMetadata, len(p.KeyMetadata))
	for keyID, metadata := range p.KeyMetadata {
		if _, ok := keepIDs[keyID]; !ok {
			continue
		}
		if keyMetadataEmpty(metadata) {
			continue
		}
		next[keyID] = metadata
	}
	if len(next) == 0 {
		p.KeyMetadata = nil
		return
	}
	p.KeyMetadata = next
}

func (p ProviderConfig) KeyMetadataForValue(key string) (KeyMetadata, bool) {
	return p.KeyMetadataForID(state.KeyID(key))
}

func (p ProviderConfig) KeyMetadataForID(keyID string) (KeyMetadata, bool) {
	if len(p.KeyMetadata) == 0 {
		return KeyMetadata{}, false
	}
	metadata, ok := p.KeyMetadata[keyID]
	return metadata, ok
}

func (p ProviderConfig) EnabledKeys() []string {
	keys := make([]string, 0, len(p.Keys))
	for _, key := range p.Keys {
		metadata, _ := p.KeyMetadataForValue(key)
		if metadata.Disabled {
			continue
		}
		keys = append(keys, key)
	}
	return keys
}

func (p ProviderConfig) DisabledKeyCount() int {
	count := 0
	for _, key := range p.Keys {
		metadata, _ := p.KeyMetadataForValue(key)
		if metadata.Disabled {
			count++
		}
	}
	return count
}
