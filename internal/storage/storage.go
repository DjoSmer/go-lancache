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
	Dirty         bool
}

func (s *Storage) IncreaseDownloadCount() {
	s.DownloadCount++
	s.UpdatedAt = time.Now()
	s.Dirty = true
}

func (s *Storage) GetStatus() StorageStatus {
	return s.Status
}

func (s *Storage) SetStatus(status StorageStatus) {
	s.Status = status
	s.UpdatedAt = time.Now()
	s.Dirty = true
}

func (s *Storage) IsSuccess() bool {
	return s.Status == StorageStatusSuccess
}

func (s *Storage) IsAvailable() bool {
	return s.Status == StorageStatusIdle || s.Status == StorageStatusError
}
