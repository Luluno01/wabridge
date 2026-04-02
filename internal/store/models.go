package store

import "time"

type Chat struct {
	JID             string    `gorm:"primaryKey" json:"jid"`
	Name            *string   `json:"name,omitempty"`
	LastMessageTime time.Time `gorm:"index" json:"last_message_time"`
}

type Contact struct {
	JID       string    `gorm:"primaryKey" json:"jid"`
	PhoneJID  *string   `gorm:"index" json:"phone_jid,omitempty"`
	PushName  *string   `json:"push_name,omitempty"`
	FullName  *string   `json:"full_name,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	ChatJID       string    `gorm:"primaryKey;index" json:"chat_jid"`
	Sender        string    `gorm:"not null;index" json:"sender"`
	Content       string    `json:"content"`
	Timestamp     time.Time `gorm:"not null;index" json:"timestamp"`
	IsFromMe      bool      `gorm:"not null" json:"is_from_me"`
	MediaType     *string   `json:"media_type,omitempty"`
	MimeType      *string   `json:"mime_type,omitempty"`
	Filename      *string   `json:"filename,omitempty"`
	URL           *string   `json:"url,omitempty"`
	MediaKey      []byte    `json:"-"`
	FileSHA256    []byte    `json:"-"`
	FileEncSHA256 []byte    `json:"-"`
	FileLength    *int64    `json:"file_length,omitempty"`
	MentionedJIDs *string   `json:"mentioned_jids,omitempty"`
}
