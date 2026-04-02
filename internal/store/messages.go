package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ListMessagesOpts struct {
	ChatJID string
	Sender  string
	After   *time.Time
	Before  *time.Time
	Search  string
	Limit   int
	Page    int
}

type MessageResult struct {
	Message
	ChatName   string `json:"chat_name"`
	SenderName string `json:"sender_name"`
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

func (s *Store) ListMessages(opts ListMessagesOpts) ([]MessageResult, error) {
	var results []MessageResult

	query := s.db.Table("messages").
		Select("messages.*, " +
			"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
			"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name").
		Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
		Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
		Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid")

	if opts.ChatJID != "" {
		query = query.Where("messages.chat_jid = ?", opts.ChatJID)
	}
	if opts.Sender != "" {
		like := "%" + opts.Sender + "%"
		query = query.Where("messages.sender LIKE ?", like)
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

	query = query.Order("messages.timestamp ASC")

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	query = query.Limit(limit)

	if opts.Page > 1 {
		query = query.Offset((opts.Page - 1) * limit)
	}

	return results, query.Scan(&results).Error
}

func (s *Store) GetMessageContext(id, chatJID string, beforeCount, afterCount int) ([]MessageResult, error) {
	var target Message
	if err := s.db.Where("id = ? AND chat_jid = ?", id, chatJID).First(&target).Error; err != nil {
		return nil, fmt.Errorf("message not found: %w", err)
	}

	baseSelect := "messages.*, " +
		"COALESCE(chats.name, ct_chat.full_name, ct_chat.push_name, messages.chat_jid) AS chat_name, " +
		"COALESCE(ct_sender.full_name, ct_sender.push_name, messages.sender) AS sender_name"

	baseJoins := func(q *gorm.DB) *gorm.DB {
		return q.Joins("LEFT JOIN chats ON messages.chat_jid = chats.jid").
			Joins("LEFT JOIN contacts ct_chat ON messages.chat_jid = ct_chat.jid").
			Joins("LEFT JOIN contacts ct_sender ON messages.sender = ct_sender.jid")
	}

	// Before (including target)
	var before []MessageResult
	q := s.db.Table("messages").Select(baseSelect).
		Where("messages.chat_jid = ? AND messages.timestamp <= ?", chatJID, target.Timestamp).
		Order("messages.timestamp DESC").
		Limit(beforeCount + 1)
	if err := baseJoins(q).Scan(&before).Error; err != nil {
		return nil, fmt.Errorf("failed to query before context: %w", err)
	}

	for i, j := 0, len(before)-1; i < j; i, j = i+1, j-1 {
		before[i], before[j] = before[j], before[i]
	}

	// After (excluding target)
	var after []MessageResult
	q = s.db.Table("messages").Select(baseSelect).
		Where("messages.chat_jid = ? AND messages.timestamp > ?", chatJID, target.Timestamp).
		Order("messages.timestamp ASC").
		Limit(afterCount)
	if err := baseJoins(q).Scan(&after).Error; err != nil {
		return nil, fmt.Errorf("failed to query after context: %w", err)
	}

	return append(before, after...), nil
}

func (s *Store) GetContactChats(contactJID string, limit int) ([]ChatResult, error) {
	var results []ChatResult

	err := s.db.Table("chats").
		Select("DISTINCT chats.*, "+
			"COALESCE(chats.name, contacts.full_name, contacts.push_name, chats.jid) AS display_name").
		Joins("LEFT JOIN contacts ON chats.jid = contacts.jid").
		Joins("INNER JOIN messages ON messages.chat_jid = chats.jid").
		Where("messages.sender = ? OR messages.chat_jid LIKE ?", contactJID, "%"+contactJID+"%").
		Order("chats.last_message_time DESC").
		Limit(limit).
		Scan(&results).Error

	return results, err
}
