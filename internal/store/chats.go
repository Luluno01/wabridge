package store

import (
	"time"

	"gorm.io/gorm/clause"
)

func (s *Store) UpsertChat(jid string, name *string, lastMessageTime time.Time) error {
	chat := Chat{
		JID:             jid,
		Name:            name,
		LastMessageTime: lastMessageTime,
	}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "jid"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "last_message_time"}),
	}).Create(&chat).Error
}

func (s *Store) GetChat(jid string) (*Chat, error) {
	var chat Chat
	if err := s.db.Where("jid = ?", jid).First(&chat).Error; err != nil {
		return nil, err
	}
	return &chat, nil
}

type ChatResult struct {
	Chat
	DisplayName string `json:"display_name"`
}

func (s *Store) ListChats(filter string, limit int) ([]ChatResult, error) {
	var results []ChatResult

	query := s.db.Table("chats").
		Select("chats.*, " +
			"COALESCE(chats.name, contacts.full_name, contacts.push_name, chats.jid) AS display_name").
		Joins("LEFT JOIN contacts ON chats.jid = contacts.jid").
		Order("chats.last_message_time DESC")

	if filter != "" {
		like := "%" + filter + "%"
		query = query.Where(
			"chats.name LIKE ? OR contacts.full_name LIKE ? OR contacts.push_name LIKE ? OR chats.jid LIKE ?",
			like, like, like, like,
		)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}

	return results, query.Scan(&results).Error
}
