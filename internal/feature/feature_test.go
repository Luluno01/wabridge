package feature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfig_Level0(t *testing.T) {
	cfg, err := NewConfig(0, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: false, HistorySync: false}, cfg)
}

func TestNewConfig_Level1(t *testing.T) {
	cfg, err := NewConfig(1, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_Level2(t *testing.T) {
	cfg, err := NewConfig(2, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_Level3(t *testing.T) {
	cfg, err := NewConfig(3, "")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: true, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_InvalidLevel(t *testing.T) {
	_, err := NewConfig(4, "")
	assert.Error(t, err)

	_, err = NewConfig(-1, "")
	assert.Error(t, err)
}

func TestNewConfig_OverrideGrant(t *testing.T) {
	// Level 0 + grant download
	cfg, err := NewConfig(0, "+download")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_OverrideRevoke(t *testing.T) {
	// Level 3 + revoke send
	cfg, err := NewConfig(3, "-send")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_MultipleOverrides(t *testing.T) {
	// Level 0 + grant download and history-sync
	cfg, err := NewConfig(0, "+download,+history-sync")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: true}, cfg)
}

func TestNewConfig_MixedOverrides(t *testing.T) {
	// Level 3 + revoke send, revoke history-sync
	cfg, err := NewConfig(3, "-send,-history-sync")
	require.NoError(t, err)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, cfg)
}

func TestNewConfig_InvalidFeatureName(t *testing.T) {
	_, err := NewConfig(3, "+invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown feature")
}

func TestNewConfig_MissingPrefix(t *testing.T) {
	_, err := NewConfig(3, "send")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must start with")
}

func TestIntersect(t *testing.T) {
	a := Config{Send: true, Download: true, HistorySync: false}
	b := Config{Send: false, Download: true, HistorySync: true}
	got := Intersect(a, b)
	assert.Equal(t, Config{Send: false, Download: true, HistorySync: false}, got)
}

func TestIntersect_BothFull(t *testing.T) {
	a := Config{Send: true, Download: true, HistorySync: true}
	b := Config{Send: true, Download: true, HistorySync: true}
	got := Intersect(a, b)
	assert.Equal(t, Config{Send: true, Download: true, HistorySync: true}, got)
}

func TestIntersect_BothEmpty(t *testing.T) {
	a := Config{}
	b := Config{}
	got := Intersect(a, b)
	assert.Equal(t, Config{}, got)
}
