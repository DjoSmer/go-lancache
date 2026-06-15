package storage

import (
	"errors"
	"log/slog"
	"os"
	"path"
	"syscall"
)

type StorageFile struct {
	fs      *os.File
	isWrite bool
}

func NewStorageFile(s *Storage, storagePath string, write bool) *StorageFile {
	op := "StorageFile"
	log := slog.Default().With(slog.String("op", op))

	basePath := s.BasePath
	ext := path.Ext(basePath)

	if ext == "" {
		basePath = basePath + ".file"
	}

	filepath := path.Join(storagePath, basePath)

	if s.IsNew {
		dir := path.Dir(filepath)
		err := os.MkdirAll(dir, 0775)
		if errors.Is(err, syscall.ENOTDIR) {
			log.Warn("Is not dir: " + dir)
			// TODO if it's not dir should delete this?
			// err := os.Remove(dir)
			// if err != nil {
			// 	log.Warn("Cannot remove: " + dir)
			// 	return nil
			// }
			// err = os.MkdirAll(dir, os.ModeDir)
			// if err != nil {
			// 	log.Warn("Cannot mkdir: " + dir)
			// 	return nil
			// }
		}
	}

	flag := os.O_RDONLY
	if write {
		flag = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	}

	fs, err := os.OpenFile(filepath, flag, 0666)
	if err != nil {
		log.Warn("Open file %s: %w", filepath, err)
		return nil
	}

	return &StorageFile{
		fs:      fs,
		isWrite: write,
	}
}

func (this *StorageFile) Read(b []byte) (int, error) {
	n, err := this.fs.Read(b)
	return n, err
}

func (this *StorageFile) Write(b []byte) (int, error) {
	n, err := this.fs.Write(b)
	return n, err
}

func (this *StorageFile) Close(err error) {
	this.fs.Close()
}
