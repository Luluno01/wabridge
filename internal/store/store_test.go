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

func TestUpsertContact_Insert(t *testing.T) {
	s := newTestStore(t)

	err := s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Alice"),
		FullName: strPtr("Alice Smith"),
	})
	require.NoError(t, err)

	c, err := s.GetContact("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Equal(t, "Alice", *c.PushName)
	assert.Equal(t, "Alice Smith", *c.FullName)
}

func TestUpsertContact_OnlyUpdatesNonEmpty(t *testing.T) {
	s := newTestStore(t)

	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		FullName: strPtr("Alice Smith"),
	})

	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Ali"),
	})

	c, err := s.GetContact("123@s.whatsapp.net")
	require.NoError(t, err)
	assert.Equal(t, "Alice Smith", *c.FullName)
	assert.Equal(t, "Ali", *c.PushName)
}

func TestUpsertContact_DualEntry(t *testing.T) {
	s := newTestStore(t)

	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Alice"),
		FullName: strPtr("Alice Smith"),
	})

	s.UpsertContact(&Contact{
		JID:      "456@lid",
		PhoneJID: strPtr("123@s.whatsapp.net"),
		PushName: strPtr("Alice"),
	})

	c, err := s.GetContact("456@lid")
	require.NoError(t, err)
	assert.Equal(t, "Alice", *c.PushName)
	assert.Equal(t, "123@s.whatsapp.net", *c.PhoneJID)
}

func TestSearchContacts(t *testing.T) {
	s := newTestStore(t)

	s.UpsertContact(&Contact{JID: "1@s.whatsapp.net", FullName: strPtr("Alice Smith")})
	s.UpsertContact(&Contact{JID: "2@s.whatsapp.net", FullName: strPtr("Bob Jones")})
	s.UpsertContact(&Contact{JID: "3@s.whatsapp.net", PushName: strPtr("Alice W")})

	results, err := s.SearchContacts("alice", 50)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestGetContactName(t *testing.T) {
	s := newTestStore(t)

	s.UpsertContact(&Contact{
		JID:      "123@s.whatsapp.net",
		PushName: strPtr("Ali"),
		FullName: strPtr("Alice Smith"),
	})

	name := s.GetContactName("123@s.whatsapp.net")
	assert.Equal(t, "Alice Smith", name)

	s.UpsertContact(&Contact{
		JID:      "456@s.whatsapp.net",
		PushName: strPtr("Bob"),
	})

	name = s.GetContactName("456@s.whatsapp.net")
	assert.Equal(t, "Bob", name)

	name = s.GetContactName("unknown@s.whatsapp.net")
	assert.Equal(t, "", name)
}
