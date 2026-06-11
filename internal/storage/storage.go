package storage

import "time"

type StorageFileStatus string

const (
	StorageFileStatusIdle    StorageFileStatus = "idle"
	StorageFileStatusPending StorageFileStatus = "pending"
	StorageFileStatusSuccess StorageFileStatus = "success"
	StorageFileStatusError   StorageFileStatus = "error"
	StorageFileStatusSkip    StorageFileStatus = "skip"
)

type Storage struct {
	BasePath      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DownloadCount uint16
	TargetId      uint16
	Status        StorageFileStatus
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

func (s *Storage) GetStatus() StorageFileStatus {
	return s.Status
}

func (s *Storage) SetStatus(status StorageFileStatus) {
	s.Status = status
	s.UpdatedAt = time.Now()
}

func (s *Storage) IsSuccess() bool {
	return s.Status == StorageFileStatusSuccess
}

// func (s *Storage) SetService(ss *StorageService) {
// 	s.service = ss
// }

// func (s *Storage) Save() {
// }
