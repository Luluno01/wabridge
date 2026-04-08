package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ListMessagesOpts struct {
	ChatJID       string
	Sender        string
	After         *time.Time
	Before        *time.Time
	Search        string
	Limit         int
	Page          int
	Latest        bool // if true, return most recent messages first
	ContextBefore int  // messages before the After boundary (edge extension)
	ContextAfter  int  // messages after the Before boundary (edge extension)
}

type MessageResult struct {
	Message
	ChatName   string `json:"chat_name"`
	SenderName string `json:"sender_name"`
	IsContext  bool   `json:"is_context,omitempty"`
}

func (s *Store) StoreMessage(msg *Message) error {
	// Auto-create chat entry
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&Chat{
		JID:             msg.ChatJID,
		LastMessageTime: msg.Timestamp,
	}).Error; err != nil {
		return fmt.Errorf("failed to upsert chat: %w", err)
	}

	// Update chat's last message time if this message is newer
	if err := s.db.Model(&Chat{}).
		Where("jid = ? AND last_message_time < ?", msg.ChatJID, msg.Timestamp).
		Update("last_message_time", msg.Timestamp).Error; err != nil {
		return fmt.Errorf("failed to update chat time: %w", err)
	}

	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}, {Name: "chat_jid"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"sender", "content", "timestamp", "is_from_me",
			"media_type", "mime_type", "filename", "url",
			"media_key", "file_sha256", "file_enc_sha256", "file_length",
			"mentioned_jids",
			"quoted_message_id", "quoted_sender", "quoted_content", "quoted_media_type",
		}),
	}).Create(msg).Error
}

func (s *Store) GetMessage(id, chatJID string) (*Message, error) {
	var msg Message
	if err := s.db.Where("id = ? AND chat_jid = ?", id, chatJID).First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

// messageSelect is the common SELECT clause for message queries with display names.
const messageSelect = "messages.*, " +
	"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
	"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name"

// messageJoins applies the standard joins for message queries with display names.
func messageJoins(q *gorm.DB) *gorm.DB {
	return q.Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
		Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
		Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid")
}

func (s *Store) ListMessages(opts ListMessagesOpts) ([]MessageResult, error) {
	var results []MessageResult

	query := messageJoins(s.db.Table("messages").Select(messageSelect))

	if opts.ChatJID != "" {
		query = query.Where("messages.chat_jid = ?", opts.ChatJID)
	}
	if opts.Sender != "" {
		query = query.Where("messages.sender = ?", opts.Sender)
	}
	if opts.After != nil {
		query = query.Where("messages.timestamp >= ?", *opts.After)
	}
	if opts.Before != nil {
		query = query.Where("messages.timestamp <= ?", *opts.Before)
	}
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where("messages.content LIKE ?", like)
	}

	if opts.Latest {
		query = query.Order("messages.timestamp DESC")
	} else {
		query = query.Order("messages.timestamp ASC")
	}

	query = paginate(query, opts.Limit, opts.Page, 50)

	if err := query.Scan(&results).Error; err != nil {
		return nil, err
	}

	// Context before: messages just before the After boundary
	if opts.ContextBefore > 0 && opts.After != nil && opts.ChatJID != "" {
		n := opts.ContextBefore
		if n > 20 {
			n = 20
		}
		var ctxBefore []MessageResult
		q := messageJoins(s.db.Table("messages").Select(messageSelect)).
			Where("messages.chat_jid = ? AND messages.timestamp < ?", opts.ChatJID, *opts.After).
			Order("messages.timestamp DESC").
			Limit(n)
		if err := q.Scan(&ctxBefore).Error; err != nil {
			return nil, fmt.Errorf("context before: %w", err)
		}
		// Only reverse to ASC when not using latest (DESC) ordering
		if !opts.Latest {
			for i, j := 0, len(ctxBefore)-1; i < j; i, j = i+1, j-1 {
				ctxBefore[i], ctxBefore[j] = ctxBefore[j], ctxBefore[i]
			}
		}
		for i := range ctxBefore {
			ctxBefore[i].IsContext = true
		}
		if opts.Latest {
			results = append(results, ctxBefore...)  // append: older goes last in DESC
		} else {
			results = append(ctxBefore, results...)  // prepend: older goes first in ASC
		}
	}

	// Context after: messages just after the Before boundary
	if opts.ContextAfter > 0 && opts.Before != nil && opts.ChatJID != "" {
		n := opts.ContextAfter
		if n > 20 {
			n = 20
		}
		var ctxAfter []MessageResult
		q := messageJoins(s.db.Table("messages").Select(messageSelect)).
			Where("messages.chat_jid = ? AND messages.timestamp > ?", opts.ChatJID, *opts.Before).
			Order("messages.timestamp ASC").
			Limit(n)
		if err := q.Scan(&ctxAfter).Error; err != nil {
			return nil, fmt.Errorf("context after: %w", err)
		}
		// Reverse to DESC when using latest ordering
		if opts.Latest {
			for i, j := 0, len(ctxAfter)-1; i < j; i, j = i+1, j-1 {
				ctxAfter[i], ctxAfter[j] = ctxAfter[j], ctxAfter[i]
			}
		}
		for i := range ctxAfter {
			ctxAfter[i].IsContext = true
		}
		if opts.Latest {
			results = append(ctxAfter, results...)  // prepend: newer goes first in DESC
		} else {
			results = append(results, ctxAfter...)  // append: newer goes last in ASC
		}
	}

	return results, nil
}

func (s *Store) GetOldestMessage(chatJID string) (*Message, error) {
	var msg Message
	if err := s.db.Select("id, chat_jid, is_from_me, timestamp").
		Where("chat_jid = ?", chatJID).Order("timestamp ASC").First(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Store) GetMessageContext(id, chatJID string, beforeCount, afterCount int) ([]MessageResult, error) {
	var target Message
	if err := s.db.Where("id = ? AND chat_jid = ?", id, chatJID).First(&target).Error; err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	// Before (including target)
	var before []MessageResult
	q := messageJoins(s.db.Table("messages").Select(messageSelect)).
		Where("messages.chat_jid = ? AND messages.timestamp <= ?", chatJID, target.Timestamp).
		Order("messages.timestamp DESC").
		Limit(beforeCount + 1)
	if err := q.Scan(&before).Error; err != nil {
		return nil, fmt.Errorf("failed to query before context: %w", err)
	}

	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		before[i], before[j] = before[j], before[i]
	}

	// After (excluding target)
	var after []MessageResult
	q = messageJoins(s.db.Table("messages").Select(messageSelect)).
		Where("messages.chat_jid = ? AND messages.timestamp > ?", chatJID, target.Timestamp).
		Order("messages.timestamp ASC").
		Limit(afterCount)
	if err := q.Scan(&after).Error; err != nil {
		return nil, fmt.Errorf("failed to query after context: %w", err)
	}

	return append(before, after...), nil
}

func (s *Store) GetContactChats(contactJID string, limit int) ([]ChatResult, error) {
	var results []ChatResult

	err := s.db.Table("chats").
		Select("DISTINCT chats.*, " + chatDisplayName).
		Joins("LEFT JOIN contacts ON chats.jid = contacts.jid").
		Joins("INNER JOIN messages ON messages.chat_jid = chats.jid").
		Where("messages.sender = ? OR messages.chat_jid LIKE ?", contactJID, "%"+contactJID+"%").
		Order("chats.last_message_time DESC").
		Limit(limit).
		Scan(&results).Error

	return results, err
}
