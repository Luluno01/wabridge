package store

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Store struct {
	db *gorm.DB
}

func New(dsn string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&Chat{}, &Contact{}, &Message{}); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// paginate applies limit and offset to a GORM query.
// Limit defaults to defaultLimit when <= 0. Page is 1-indexed.
func paginate(q *gorm.DB, limit, page, defaultLimit int) *gorm.DB {
	if limit <= 0 {
		limit = defaultLimit
	}
	q = q.Limit(limit)
	if page > 1 {
		q = q.Offset((page - 1) * limit)
	}
	return q
}

func (s *Store) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
