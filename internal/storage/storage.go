package storage

import (
	"time"
)

type StorageStatus string

const (
	StorageStatusIdle    StorageStatus = "idle"
	StorageStatusPending StorageStatus = "pending"
	StorageStatusSuccess StorageStatus = "success"
	StorageStatusError   StorageStatus = "error"
	StorageStatusSkip    StorageStatus = "skip"
)

type Storage struct {
	BasePath      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DownloadCount int
	TargetId      int
	Status        StorageStatus
	IsNew         bool
	// service       *StorageService
}

// func (s *Storage) AddInstance() {}
// func (s *Storage) InstanceClose() {}
// func (s *Storage) close() {}

func (s *Storage) IncreaseDownloadCount() {
	s.DownloadCount++
	s.UpdatedAt = time.Now()
}

func (s *Storage) GetStatus() StorageStatus {
	return s.Status
}

func (s *Storage) SetStatus(status StorageStatus) {
	s.Status = status
	s.UpdatedAt = time.Now()
}

func (s *Storage) IsSuccess() bool {
	return s.Status == StorageStatusSuccess
}

func (s *Storage) IsAvailable() bool {
	return s.Status == StorageStatusIdle || s.Status == StorageStatusError
}

// func (s *Storage) SetService(ss *StorageService) {
// 	s.service = ss
// }

// func (s *Storage) Save() {
// }
