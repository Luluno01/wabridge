package store

import "time"

type Chat struct {
	JID             string    `gorm:"column:jid;primaryKey" json:"jid"`
	Name            *string   `json:"name,omitempty"`
	LastMessageTime time.Time `gorm:"index" json:"last_message_time"`
}

type Contact struct {
	JID       string    `gorm:"column:jid;primaryKey" json:"jid"`
	PhoneJID  *string   `gorm:"column:phone_jid;index" json:"phone_jid,omitempty"`
	PushName  *string   `json:"push_name,omitempty"`
	FullName  *string   `json:"full_name,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	ChatJID       string    `gorm:"column:chat_jid;primaryKey;index" json:"chat_jid"`
	Sender        string    `gorm:"not null;index" json:"sender"`
	Content       string    `json:"content"`
	Timestamp     time.Time `gorm:"not null;index" json:"timestamp"`
	IsFromMe      bool      `gorm:"not null" json:"is_from_me"`
	MediaType     *string   `json:"media_type,omitempty"`
	MimeType      *string   `json:"mime_type,omitempty"`
	Filename      *string   `json:"filename,omitempty"`
	URL           *string   `json:"url,omitempty"`
	MediaKey      []byte    `json:"-"`
	FileSHA256    []byte    `gorm:"column:file_sha256" json:"-"`
	FileEncSHA256 []byte    `gorm:"column:file_enc_sha256" json:"-"`
	FileLength    *int64    `json:"file_length,omitempty"`
	MentionedJIDs   *string `gorm:"column:mentioned_jids" json:"mentioned_jids,omitempty"`
	QuotedMessageID *string `gorm:"column:quoted_message_id" json:"quoted_message_id,omitempty"`
	QuotedSender    *string `gorm:"column:quoted_sender" json:"quoted_sender,omitempty"`
	QuotedContent   *string `gorm:"column:quoted_content" json:"quoted_content,omitempty"`
	QuotedMediaType *string `gorm:"column:quoted_media_type" json:"quoted_media_type,omitempty"`
}
