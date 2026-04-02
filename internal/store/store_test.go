package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func strPtr(s string) *string { return &s }

func TestUpsertChat(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	err := s.UpsertChat("group@g.us", strPtr("Family Group"), now)
	require.NoError(t, err)

	chat, err := s.GetChat("group@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Family Group", *chat.Name)

	err = s.UpsertChat("group@g.us", strPtr("Renamed Group"), now.Add(time.Hour))
	require.NoError(t, err)

	chat, err = s.GetChat("group@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Renamed Group", *chat.Name)
}

func TestUpsertChat_NilName(t *testing.T) {
	s := newTestStore(t)

	err := s.UpsertChat("123@s.whatsapp.net", nil, time.Now())
	require.NoError(t, err)

	chat, err := s.GetChat("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Nil(t, chat.Name)
}

func TestListChats(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	s.UpsertChat("a@g.us", strPtr("Alpha"), now)
	s.UpsertChat("b@g.us", strPtr("Beta"), now.Add(time.Hour))
	s.UpsertChat("c@s.whatsapp.net", nil, now.Add(2*time.Hour))

	chats, err := s.ListChats("", 10)
	require.NoError(t, err)
	assert.Len(t, chats, 3)
	assert.Equal(t, "c@s.whatsapp.net", chats[0].JID) // most recent first

	chats, err = s.ListChats("alpha", 10)
	require.NoError(t, err)
	assert.Len(t, chats, 1)
	assert.Equal(t, "a@g.us", chats[0].JID)
}

func TestGetChat_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetChat("nonexistent@g.us")
	assert.Error(t, err)
}
