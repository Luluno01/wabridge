package store

import (
	"fmt"
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

	chats, err := s.ListChats("", 10, 0)
	require.NoError(t, err)
	assert.Len(t, chats, 3)
	assert.Equal(t, "c@s.whatsapp.net", chats[0].JID) // most recent first

	chats, err = s.ListChats("alpha", 10, 0)
	require.NoError(t, err)
	assert.Len(t, chats, 1)
	assert.Equal(t, "a@g.us", chats[0].JID)
}

func TestListChats_Pagination(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		s.UpsertChat(fmt.Sprintf("chat%d@g.us", i), strPtr(fmt.Sprintf("Chat %d", i)), now.Add(time.Duration(i)*time.Hour))
	}

	// Page 1, limit 2 — most recent first
	chats, err := s.ListChats("", 2, 1)
	require.NoError(t, err)
	assert.Len(t, chats, 2)
	assert.Equal(t, "chat4@g.us", chats[0].JID)
	assert.Equal(t, "chat3@g.us", chats[1].JID)

	// Page 2
	chats, err = s.ListChats("", 2, 2)
	require.NoError(t, err)
	assert.Len(t, chats, 2)
	assert.Equal(t, "chat2@g.us", chats[0].JID)
	assert.Equal(t, "chat1@g.us", chats[1].JID)

	// Page 3 — only 1 remaining
	chats, err = s.ListChats("", 2, 3)
	require.NoError(t, err)
	assert.Len(t, chats, 1)
	assert.Equal(t, "chat0@g.us", chats[0].JID)
}

func TestListChats_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Second)

	for i := 0; i < 25; i++ {
		s.UpsertChat(fmt.Sprintf("chat%02d@g.us", i), strPtr(fmt.Sprintf("Chat %02d", i)), now.Add(time.Duration(i)*time.Hour))
	}

	// No explicit limit defaults to 20
	chats, err := s.ListChats("", 0, 0)
	require.NoError(t, err)
	assert.Len(t, chats, 20)

	// Page 2 gets the remaining 5
	chats, err = s.ListChats("", 0, 2)
	require.NoError(t, err)
	assert.Len(t, chats, 5)
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

func TestStoreMessage(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "msg1",
		ChatJID:   "chat@g.us",
		Sender:    "123@s.whatsapp.net",
		Content:   "Hello world",
		Timestamp: time.Now().Truncate(time.Second),
		IsFromMe:  false,
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	msg.Content = "Updated content"
	err = s.StoreMessage(msg)
	require.NoError(t, err)

	got, err := s.GetMessage("msg1", "chat@g.us")
	require.NoError(t, err)
	assert.Equal(t, "Updated content", got.Content)
}

func TestStoreMessage_CreatesChat(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "msg1",
		ChatJID:   "new-chat@g.us",
		Sender:    "123@s.whatsapp.net",
		Content:   "First message",
		Timestamp: time.Now(),
		IsFromMe:  false,
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	chat, err := s.GetChat("new-chat@g.us")
	require.NoError(t, err)
	assert.Equal(t, "new-chat@g.us", chat.JID)
}

func TestListMessages(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	s.UpsertContact(&Contact{JID: "alice@s.whatsapp.net", FullName: strPtr("Alice Smith")})
	s.UpsertChat("chat@g.us", strPtr("Test Group"), base.Add(2*time.Minute))

	for i := 0; i < 5; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 5)
	assert.Equal(t, "Alice Smith", results[0].SenderName)
	assert.Equal(t, "Test Group", results[0].ChatName)

	after := base.Add(2 * time.Minute)
	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", After: &after, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 3)

	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Search: "Message 3", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, results, 1)

	results, err = s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 2, Page: 2})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestStoreMessage_QuotedFields(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:              "reply1",
		ChatJID:         "chat@g.us",
		Sender:          "alice@s.whatsapp.net",
		Content:         "I agree!",
		Timestamp:       time.Now().Truncate(time.Second),
		QuotedMessageID: strPtr("orig1"),
		QuotedSender:    strPtr("bob@s.whatsapp.net"),
		QuotedContent:   strPtr("What do you think?"),
	}

	err := s.StoreMessage(msg)
	require.NoError(t, err)

	got, err := s.GetMessage("reply1", "chat@g.us")
	require.NoError(t, err)
	require.NotNil(t, got.QuotedMessageID)
	assert.Equal(t, "orig1", *got.QuotedMessageID)
	require.NotNil(t, got.QuotedSender)
	assert.Equal(t, "bob@s.whatsapp.net", *got.QuotedSender)
	require.NotNil(t, got.QuotedContent)
	assert.Equal(t, "What do you think?", *got.QuotedContent)
	assert.Nil(t, got.QuotedMediaType)
}

func TestStoreMessage_QuotedFieldsUpsert(t *testing.T) {
	s := newTestStore(t)

	msg := &Message{
		ID:        "reply1",
		ChatJID:   "chat@g.us",
		Sender:    "alice@s.whatsapp.net",
		Content:   "I agree!",
		Timestamp: time.Now().Truncate(time.Second),
	}
	require.NoError(t, s.StoreMessage(msg))

	// Re-store with quoted fields populated (simulates re-sync)
	msg.QuotedMessageID = strPtr("orig1")
	msg.QuotedSender = strPtr("bob@s.whatsapp.net")
	msg.QuotedContent = strPtr("What do you think?")
	msg.QuotedMediaType = strPtr("image")
	require.NoError(t, s.StoreMessage(msg))

	got, err := s.GetMessage("reply1", "chat@g.us")
	require.NoError(t, err)
	require.NotNil(t, got.QuotedMessageID)
	assert.Equal(t, "orig1", *got.QuotedMessageID)
	require.NotNil(t, got.QuotedSender)
	assert.Equal(t, "bob@s.whatsapp.net", *got.QuotedSender)
	require.NotNil(t, got.QuotedContent)
	assert.Equal(t, "What do you think?", *got.QuotedContent)
	require.NotNil(t, got.QuotedMediaType)
	assert.Equal(t, "image", *got.QuotedMediaType)
}

func TestListMessages_IncludesQuotedFields(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	s.StoreMessage(&Message{
		ID:        "orig1",
		ChatJID:   "chat@g.us",
		Sender:    "bob@s.whatsapp.net",
		Content:   "What do you think?",
		Timestamp: base,
	})
	s.StoreMessage(&Message{
		ID:              "reply1",
		ChatJID:         "chat@g.us",
		Sender:          "alice@s.whatsapp.net",
		Content:         "I agree!",
		Timestamp:       base.Add(time.Minute),
		QuotedMessageID: strPtr("orig1"),
		QuotedSender:    strPtr("bob@s.whatsapp.net"),
		QuotedContent:   strPtr("What do you think?"),
	})

	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	require.Len(t, results, 2)

	// First message (orig) has no quoted fields
	assert.Nil(t, results[0].QuotedMessageID)

	// Second message (reply) has quoted fields
	require.NotNil(t, results[1].QuotedMessageID)
	assert.Equal(t, "orig1", *results[1].QuotedMessageID)
	require.NotNil(t, results[1].QuotedSender)
	assert.Equal(t, "bob@s.whatsapp.net", *results[1].QuotedSender)
	require.NotNil(t, results[1].QuotedContent)
	assert.Equal(t, "What do you think?", *results[1].QuotedContent)
}

func TestListMessages_ContextEdges(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	// Without context params, IsContext should be false on all results
	results, err := s.ListMessages(ListMessagesOpts{ChatJID: "chat@g.us", Limit: 10})
	require.NoError(t, err)
	for _, r := range results {
		assert.False(t, r.IsContext)
	}

	// context_before: 2 messages before the After boundary
	after := base.Add(5 * time.Minute) // msg5 is first match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after,
		ContextBefore: 2,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 7) // 2 context (msg3,msg4) + 5 matched (msg5..msg9)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg3", results[0].ID)
	assert.True(t, results[1].IsContext)
	assert.Equal(t, "msg4", results[1].ID)
	assert.False(t, results[2].IsContext)
	assert.Equal(t, "msg5", results[2].ID)

	// context_after: 2 messages after the Before boundary
	before := base.Add(4 * time.Minute) // msg4 is last match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:      "chat@g.us",
		Before:       &before,
		ContextAfter: 2,
		Limit:        10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 7) // 5 matched (msg0..msg4) + 2 context (msg5,msg6)
	assert.False(t, results[4].IsContext)
	assert.Equal(t, "msg4", results[4].ID)
	assert.True(t, results[5].IsContext)
	assert.Equal(t, "msg5", results[5].ID)
	assert.True(t, results[6].IsContext)
	assert.Equal(t, "msg6", results[6].ID)

	// Both context_before and context_after
	after2 := base.Add(3 * time.Minute) // msg3 is first match
	before2 := base.Add(6 * time.Minute) // msg6 is last match
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after2,
		Before:        &before2,
		ContextBefore: 2,
		ContextAfter:  2,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 8) // 2 ctx (msg1,msg2) + 4 match (msg3..msg6) + 2 ctx (msg7,msg8)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg1", results[0].ID)
	assert.True(t, results[1].IsContext)
	assert.Equal(t, "msg2", results[1].ID)
	assert.False(t, results[2].IsContext)
	assert.False(t, results[3].IsContext)
	assert.False(t, results[4].IsContext)
	assert.Equal(t, "msg6", results[5].ID)
	assert.True(t, results[6].IsContext)
	assert.Equal(t, "msg7", results[6].ID)
	assert.True(t, results[7].IsContext)
	assert.Equal(t, "msg8", results[7].ID)

	// context_before without After is a no-op
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		ContextBefore: 3,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10)
	for _, r := range results {
		assert.False(t, r.IsContext)
	}

	// context_after without Before is a no-op
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:      "chat@g.us",
		ContextAfter: 3,
		Limit:        10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10)
	for _, r := range results {
		assert.False(t, r.IsContext)
	}

	// Fewer context messages available than requested
	after3 := base.Add(1 * time.Minute) // msg1 is first match — only msg0 before it
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after3,
		ContextBefore: 5,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 10) // 1 context (msg0) + 9 matched (msg1..msg9)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg0", results[0].ID)
	assert.False(t, results[1].IsContext)

	// context with latest=true — entire result in DESC order
	after4 := base.Add(3 * time.Minute)
	before4 := base.Add(6 * time.Minute)
	results, err = s.ListMessages(ListMessagesOpts{
		ChatJID:       "chat@g.us",
		After:         &after4,
		Before:        &before4,
		ContextBefore: 2,
		ContextAfter:  2,
		Latest:        true,
		Limit:         10,
	})
	require.NoError(t, err)
	assert.Len(t, results, 8) // 2 ctx_after (msg8,msg7) + 4 match (msg6..msg3) + 2 ctx_before (msg2,msg1)
	// context_after in DESC (newest first)
	assert.True(t, results[0].IsContext)
	assert.Equal(t, "msg8", results[0].ID)
	assert.True(t, results[1].IsContext)
	assert.Equal(t, "msg7", results[1].ID)
	// matched in DESC
	assert.False(t, results[2].IsContext)
	assert.Equal(t, "msg6", results[2].ID)
	assert.False(t, results[5].IsContext)
	assert.Equal(t, "msg3", results[5].ID)
	// context_before in DESC (newest of the older first)
	assert.True(t, results[6].IsContext)
	assert.Equal(t, "msg2", results[6].ID)
	assert.True(t, results[7].IsContext)
	assert.Equal(t, "msg1", results[7].ID)
}

func TestGetMessageContext(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 10; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	results, err := s.GetMessageContext("msg5", "chat@g.us", 2, 2)
	require.NoError(t, err)
	assert.Len(t, results, 5) // msg3, msg4, msg5, msg6, msg7
	assert.Equal(t, "Message 3", results[0].Content)
	assert.Equal(t, "Message 7", results[4].Content)
}

func TestGetOldestMessage(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		s.StoreMessage(&Message{
			ID:        fmt.Sprintf("msg%d", i),
			ChatJID:   "chat@g.us",
			Sender:    "alice@s.whatsapp.net",
			Content:   fmt.Sprintf("Message %d", i),
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			IsFromMe:  i%2 == 0,
		})
	}

	msg, err := s.GetOldestMessage("chat@g.us")
	require.NoError(t, err)
	assert.Equal(t, "msg0", msg.ID)
	assert.Equal(t, "chat@g.us", msg.ChatJID)
	assert.True(t, msg.IsFromMe)
}

func TestGetOldestMessage_IsolatedByChat(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)

	s.StoreMessage(&Message{
		ID: "older", ChatJID: "chat-a@g.us", Sender: "alice@s.whatsapp.net",
		Content: "older in A", Timestamp: base,
	})
	s.StoreMessage(&Message{
		ID: "newer", ChatJID: "chat-b@g.us", Sender: "bob@s.whatsapp.net",
		Content: "newer in B", Timestamp: base.Add(time.Hour),
	})

	msg, err := s.GetOldestMessage("chat-b@g.us")
	require.NoError(t, err)
	assert.Equal(t, "newer", msg.ID)
	assert.Equal(t, "chat-b@g.us", msg.ChatJID)
}

func TestGetOldestMessage_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetOldestMessage("nonexistent@g.us")
	assert.Error(t, err)
}
