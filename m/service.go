package m

import (
	"gorm.io/gorm"
)

// Service represents the m web service.
type Service struct {
	db *gorm.DB
}

// NewService returns a new Service.
func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
	}
}

func (s *Service) DB() *gorm.DB {
	return s.db
}

func (s *Service) Instances() *instances {
	return &instances{
		db: s.db,
	}
}
