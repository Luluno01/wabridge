package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	appstore "wabridge/internal/store"
)

// RegisterHandlers registers the event handler on the whatsmeow client.
func (c *Client) RegisterHandlers() {
	c.WAClient.AddEventHandler(c.handleEvent)
}

// handleEvent dispatches incoming whatsmeow events to dedicated handlers.
func (c *Client) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		c.handleMessage(v)
	case *events.HistorySync:
		c.handleHistorySync(v)
	case *events.PushName:
		c.handlePushName(v)
	case *events.Contact:
		c.handleContact(v)
	case *events.Connected:
		c.handleConnected()
	case *events.LoggedOut:
		c.Log.Warnf("Device logged out, please scan QR code to log in again")
	}
}

// handleMessage processes a real-time incoming or outgoing message.
func (c *Client) handleMessage(msg *events.Message) {
	chatJID := msg.Info.Chat.String()
	sender := msg.Info.Sender.ToNonAD().String()

	if msg.Info.PushName != "" && !msg.Info.IsFromMe {
		if err := c.Store.UpsertContact(&appstore.Contact{
			JID:      msg.Info.Sender.String(),
			PushName: strPtr(msg.Info.PushName),
		}); err != nil {
			c.Log.Warnf("Failed to store push name from message: %v", err)
		}
	}

	name := c.resolveChatName(msg.Info.Chat, chatJID)
	if err := c.Store.UpsertChat(chatJID, name, msg.Info.Timestamp); err != nil {
		c.Log.Warnf("Failed to store chat: %v", err)
	}

	storeMsg := c.buildMessage(msg.Info.ID, chatJID, sender, msg.Message, msg.Info.Timestamp, msg.Info.IsFromMe)
	if storeMsg == nil {
		return
	}

	if err := c.Store.StoreMessage(storeMsg); err != nil {
		c.Log.Warnf("Failed to store message: %v", err)
		return
	}

	direction := "<-"
	if msg.Info.IsFromMe {
		direction = "->"
	}
	ts := msg.Info.Timestamp.Format("2006-01-02 15:04:05")
	if storeMsg.MediaType != nil {
		c.Log.Infof("[%s] %s %s: [%s] %s", ts, direction, sender, *storeMsg.MediaType, storeMsg.Content)
	} else if storeMsg.Content != "" {
		c.Log.Infof("[%s] %s %s: %s", ts, direction, sender, storeMsg.Content)
	}
}

// handleHistorySync processes a batch of historical messages from WhatsApp.
func (c *Client) handleHistorySync(historySync *events.HistorySync) {
	c.mu.Lock()
	c.lastSyncEvent = time.Now().UnixMilli()
	c.mu.Unlock()

	conversations := historySync.Data.GetConversations()
	c.Log.Infof("History sync: received %d conversations", len(conversations))

	syncedCount := 0
	for _, conversation := range conversations {
		if conversation.GetID() == "" {
			continue
		}
		chatJID := conversation.GetID()

		jid, err := types.ParseJID(chatJID)
		if err != nil {
			c.Log.Warnf("Failed to parse JID %s: %v", chatJID, err)
			continue
		}

		name := c.resolveHistoryChatName(jid, chatJID, conversation.GetDisplayName(), conversation.GetName())

		messages := conversation.GetMessages()
		if len(messages) == 0 {
			// Still upsert the chat even without messages
			if err := c.Store.UpsertChat(chatJID, name, time.Time{}); err != nil {
				c.Log.Warnf("Failed to store chat %s: %v", chatJID, err)
			}
			continue
		}

		var latestTS time.Time
		for _, msg := range messages {
			if msg == nil || msg.GetMessage() == nil {
				continue
			}
			if ts := msg.GetMessage().GetMessageTimestamp(); ts != 0 {
				t := time.Unix(int64(ts), 0)
				if t.After(latestTS) {
					latestTS = t
				}
			}
		}

		if err := c.Store.UpsertChat(chatJID, name, latestTS); err != nil {
			c.Log.Warnf("Failed to store chat %s: %v", chatJID, err)
		}

		// Parse and store individual messages
		for _, msg := range messages {
			if msg == nil || msg.GetMessage() == nil {
				continue
			}

			evt, err := c.WAClient.ParseWebMessage(jid, msg.GetMessage())
			if err != nil {
				c.Log.Warnf("Failed to parse history message: %v", err)
				continue
			}

			if evt.Info.PushName != "" && !evt.Info.IsFromMe {
				if err := c.Store.UpsertContact(&appstore.Contact{
					JID:      evt.Info.Sender.String(),
					PushName: strPtr(evt.Info.PushName),
				}); err != nil {
					c.Log.Warnf("Failed to store push name from history: %v", err)
				}
			}

			sender := evt.Info.Sender.ToNonAD().String()
			storeMsg := c.buildMessage(evt.Info.ID, chatJID, sender, evt.Message, evt.Info.Timestamp, evt.Info.IsFromMe)
			if storeMsg == nil {
				continue
			}

			if err := c.Store.StoreMessage(storeMsg); err != nil {
				c.Log.Warnf("Failed to store history message: %v", err)
				continue
			}
			syncedCount++
		}
	}

	c.Log.Infof("History sync: stored %d messages", syncedCount)

	// Start background goroutine to detect sync settlement (only once)
	c.mu.Lock()
	started := c.syncStarted
	if !started {
		c.syncStarted = true
	}
	c.mu.Unlock()
	if !started {
		go c.waitForSyncSettled()
	}
}

// waitForSyncSettled waits until no new history sync events arrive for 15
// seconds, then dumps contacts and signals settlement.
func (c *Client) waitForSyncSettled() {
	for {
		time.Sleep(15 * time.Second)
		c.mu.Lock()
		last := c.lastSyncEvent
		c.mu.Unlock()

		if time.Since(time.UnixMilli(last)) >= 15*time.Second {
			break
		}
	}

	c.Log.Infof("History sync settled, dumping contacts")
	c.dumpContacts()

	close(c.syncSettled)
}

// handlePushName stores an updated push name for a contact.
func (c *Client) handlePushName(evt *events.PushName) {
	c.Log.Infof("PushName: %s -> %s (old: %s)", evt.JID.String(), evt.NewPushName, evt.OldPushName)
	if err := c.Store.UpsertContact(&appstore.Contact{
		JID:      evt.JID.String(),
		PushName: strPtr(evt.NewPushName),
	}); err != nil {
		c.Log.Warnf("Failed to store push name: %v", err)
	}
}

// handleContact stores an updated contact full name.
func (c *Client) handleContact(evt *events.Contact) {
	if evt.Action == nil {
		return
	}
	fullName := evt.Action.GetFullName()
	firstName := evt.Action.GetFirstName()
	c.Log.Infof("Contact: %s -> full=%s, first=%s", evt.JID.String(), fullName, firstName)

	nameToStore := fullName
	if nameToStore == "" {
		nameToStore = firstName
	}
	if nameToStore == "" {
		return
	}

	if err := c.Store.UpsertContact(&appstore.Contact{
		JID:      evt.JID.String(),
		FullName: strPtr(nameToStore),
	}); err != nil {
		c.Log.Warnf("Failed to store contact: %v", err)
	}
}

// handleConnected is called when the client successfully connects to WhatsApp.
func (c *Client) handleConnected() {
	c.Log.Infof("Connected to WhatsApp")
}

// dumpContacts fetches all contacts from the whatsmeow device store and
// upserts them into the application store, including LID dual entries.
func (c *Client) dumpContacts() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	contacts, err := c.WAClient.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		c.Log.Warnf("Failed to get all contacts: %v", err)
		return
	}

	stored := 0
	for jid, info := range contacts {
		fullName := info.FullName
		if fullName == "" {
			fullName = info.BusinessName
		}
		if fullName == "" && info.PushName == "" {
			continue
		}

		phoneJID := jid.String()
		contact := &appstore.Contact{
			JID: phoneJID,
		}
		if info.PushName != "" {
			contact.PushName = strPtr(info.PushName)
		}
		if fullName != "" {
			contact.FullName = strPtr(fullName)
		}

		if err := c.Store.UpsertContact(contact); err != nil {
			c.Log.Warnf("Failed to store contact %s: %v", phoneJID, err)
			continue
		}
		stored++

		// Also create a LID dual entry so chats keyed by LID can resolve names
		lidCtx, lidCancel := context.WithTimeout(context.Background(), 5*time.Second)
		lidJID, err := c.WAClient.Store.LIDs.GetLIDForPN(lidCtx, jid)
		lidCancel()
		if err == nil && !lidJID.IsEmpty() {
			lidContact := &appstore.Contact{
				JID:      lidJID.String(),
				PhoneJID: strPtr(phoneJID),
			}
			if info.PushName != "" {
				lidContact.PushName = strPtr(info.PushName)
			}
			if fullName != "" {
				lidContact.FullName = strPtr(fullName)
			}
			if err := c.Store.UpsertContact(lidContact); err != nil {
				c.Log.Warnf("Failed to store LID contact %s: %v", lidJID.String(), err)
			}
		}
	}

	c.Log.Infof("Dumped %d contacts from device store", stored)
}

// SyncSettled returns a channel that is signalled when the history sync has
// settled (no new events for 15 seconds) and contacts have been dumped.
func (c *Client) SyncSettled() <-chan struct{} {
	return c.syncSettled
}

// resolveChatName determines the appropriate name for a chat.
// For individual chats, returns nil (display names are resolved at query time).
// For groups, checks the DB cache, then queries WhatsApp.
func (c *Client) resolveChatName(jid types.JID, chatJID string) *string {
	if jid.Server != "g.us" {
		return nil
	}

	existing, err := c.Store.GetChat(chatJID)
	if err == nil && existing.Name != nil && *existing.Name != "" && !strings.HasPrefix(*existing.Name, "Group ") {
		return existing.Name
	}
	return c.fetchGroupName(jid)
}

// resolveHistoryChatName resolves the name for a chat from history sync data.
// Prefers conversation metadata; falls back to GetGroupInfo for groups.
func (c *Client) resolveHistoryChatName(jid types.JID, chatJID, displayName, convName string) *string {
	if jid.Server != "g.us" {
		return nil
	}

	existing, err := c.Store.GetChat(chatJID)
	if err == nil && existing.Name != nil && *existing.Name != "" && !strings.HasPrefix(*existing.Name, "Group ") {
		return existing.Name
	}

	if displayName != "" {
		return strPtr(displayName)
	}
	if convName != "" {
		return strPtr(convName)
	}

	return c.fetchGroupName(jid)
}

// fetchGroupName queries WhatsApp for a group name. Callers should check
// the DB cache before calling this to avoid unnecessary network requests.
func (c *Client) fetchGroupName(jid types.JID) *string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groupInfo, err := c.WAClient.GetGroupInfo(ctx, jid)
	if err == nil && groupInfo.Name != "" {
		return strPtr(groupInfo.Name)
	}
	return strPtr(fmt.Sprintf("Group %s", jid.User))
}

// buildMessage creates a store.Message from a whatsmeow message. Returns nil
// if the message has no text content and no media (nothing to store).
func (c *Client) buildMessage(id types.MessageID, chatJID, sender string, msg *waE2E.Message, ts time.Time, isFromMe bool) *appstore.Message {
	content := extractTextContent(msg)
	mediaType, mimeType, filename, url, mediaKey, fileSHA256, fileEncSHA256, fileLength := extractMediaInfo(msg, ts)
	mentionedJIDs := extractMentionedJIDs(msg)

	if content == "" && mediaType == "" {
		return nil
	}

	storeMsg := &appstore.Message{
		ID:        string(id),
		ChatJID:   chatJID,
		Sender:    sender,
		Content:   content,
		Timestamp: ts,
		IsFromMe:  isFromMe,
	}

	if mediaType != "" {
		storeMsg.MediaType = strPtr(mediaType)
	}
	if mimeType != "" {
		storeMsg.MimeType = strPtr(mimeType)
	}
	if filename != "" {
		storeMsg.Filename = strPtr(filename)
	}
	if url != "" {
		storeMsg.URL = strPtr(url)
	}
	if mediaKey != nil {
		storeMsg.MediaKey = mediaKey
	}
	if fileSHA256 != nil {
		storeMsg.FileSHA256 = fileSHA256
	}
	if fileEncSHA256 != nil {
		storeMsg.FileEncSHA256 = fileEncSHA256
	}
	if fileLength > 0 {
		fl := int64(fileLength)
		storeMsg.FileLength = &fl
	}
	if mentionedJIDs != "" {
		storeMsg.MentionedJIDs = strPtr(mentionedJIDs)
	}

	return storeMsg
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// extractTextContent extracts the text body from a waE2E.Message.
// It checks: Conversation > ExtendedTextMessage > media captions.
func extractTextContent(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	if text := msg.GetConversation(); text != "" {
		return text
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText()
	}

	// Media captions
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption()
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetCaption()
	}

	return ""
}

// extractMediaInfo extracts media metadata from a message without downloading
// the media itself. Uses the message timestamp for fallback filenames.
// Returns empty values if the message has no media.
func extractMediaInfo(msg *waE2E.Message, ts time.Time) (mediaType, mimeType, filename, url string, mediaKey, fileSHA256, fileEncSHA256 []byte, fileLength uint64) {
	if msg == nil {
		return
	}

	tsStr := ts.Format("20060102_150405")

	if img := msg.GetImageMessage(); img != nil {
		return "image", img.GetMimetype(),
			"image_" + tsStr + ".jpg",
			img.GetURL(), img.GetMediaKey(), img.GetFileSHA256(), img.GetFileEncSHA256(), img.GetFileLength()
	}

	if vid := msg.GetVideoMessage(); vid != nil {
		return "video", vid.GetMimetype(),
			"video_" + tsStr + ".mp4",
			vid.GetURL(), vid.GetMediaKey(), vid.GetFileSHA256(), vid.GetFileEncSHA256(), vid.GetFileLength()
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		return "audio", aud.GetMimetype(),
			"audio_" + tsStr + ".ogg",
			aud.GetURL(), aud.GetMediaKey(), aud.GetFileSHA256(), aud.GetFileEncSHA256(), aud.GetFileLength()
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		fn := doc.GetFileName()
		if fn == "" {
			fn = "document_" + tsStr
		}
		return "document", doc.GetMimetype(), fn,
			doc.GetURL(), doc.GetMediaKey(), doc.GetFileSHA256(), doc.GetFileEncSHA256(), doc.GetFileLength()
	}

	if stk := msg.GetStickerMessage(); stk != nil {
		return "sticker", stk.GetMimetype(),
			"sticker_" + tsStr + ".webp",
			stk.GetURL(), stk.GetMediaKey(), stk.GetFileSHA256(), stk.GetFileEncSHA256(), stk.GetFileLength()
	}

	return
}

// extractMentionedJIDs collects @-mentioned JIDs from a message's ContextInfo
// and returns them as a comma-separated string. Returns "" if none.
func extractMentionedJIDs(msg *waE2E.Message) string {
	if msg == nil {
		return ""
	}

	var ctx *waE2E.ContextInfo

	if ext := msg.GetExtendedTextMessage(); ext != nil {
		ctx = ext.GetContextInfo()
	} else if img := msg.GetImageMessage(); img != nil {
		ctx = img.GetContextInfo()
	} else if vid := msg.GetVideoMessage(); vid != nil {
		ctx = vid.GetContextInfo()
	} else if doc := msg.GetDocumentMessage(); doc != nil {
		ctx = doc.GetContextInfo()
	} else if aud := msg.GetAudioMessage(); aud != nil {
		ctx = aud.GetContextInfo()
	}

	if ctx == nil {
		return ""
	}

	jids := ctx.GetMentionedJID()
	if len(jids) == 0 {
		return ""
	}

	data, _ := json.Marshal(jids)
	return string(data)
}

// strPtr returns a pointer to the given string. Convenience helper to avoid
// temporary variables when populating optional string fields.
func strPtr(s string) *string {
	return &s
}
