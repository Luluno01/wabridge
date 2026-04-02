package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

func (s *Store) UpsertContact(contact *Contact) error {
	contact.UpdatedAt = time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		var existing Contact
		err := tx.Where("jid = ?", contact.JID).First(&existing).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(contact).Error
		}
		if err != nil {
			return err
		}

		updates := map[string]interface{}{
			"updated_at": contact.UpdatedAt,
		}
		if contact.PhoneJID != nil && *contact.PhoneJID != "" {
			updates["phone_jid"] = *contact.PhoneJID
		}
		if contact.PushName != nil && *contact.PushName != "" {
			updates["push_name"] = *contact.PushName
		}
		if contact.FullName != nil && *contact.FullName != "" {
			updates["full_name"] = *contact.FullName
		}

		return tx.Model(&existing).Updates(updates).Error
	})
}

func (s *Store) ClearContactField(jid, field string) error {
	return s.db.Model(&Contact{}).Where("jid = ?", jid).Update(field, nil).Error
}

func (s *Store) GetContact(jid string) (*Contact, error) {
	var contact Contact
	if err := s.db.Where("jid = ?", jid).First(&contact).Error; err != nil {
		return nil, err
	}
	return &contact, nil
}

func (s *Store) GetContactName(jid string) string {
	var contact Contact
	err := s.db.Where("jid = ? OR phone_jid = ?", jid, jid).First(&contact).Error
	if err != nil {
		return ""
	}
	if contact.FullName != nil && *contact.FullName != "" {
		return *contact.FullName
	}
	if contact.PushName != nil && *contact.PushName != "" {
		return *contact.PushName
	}
	return ""
}

func (s *Store) SearchContacts(query string, limit int) ([]Contact, error) {
	var contacts []Contact
	like := "%" + query + "%"

	err := s.db.Where(
		"full_name LIKE ? OR push_name LIKE ? OR jid LIKE ?",
		like, like, like,
	).Where("jid NOT LIKE ?", "%@g.us").
		Limit(limit).
		Find(&contacts).Error

	return contacts, err
}
