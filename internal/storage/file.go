package storage

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path"
	"syscall"
)

type StorageFile interface {
	io.ReadWriteCloser
	Remove() error
}

type storageFile struct {
	fs       *os.File
	isWrite  bool
	filepath string
}

func NewStorageFile(s *Storage, storagePath string, write bool) *storageFile {
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

	return &storageFile{
		fs:       fs,
		isWrite:  write,
		filepath: filepath,
	}
}

func (f *storageFile) Read(b []byte) (int, error) {
	n, err := f.fs.Read(b)
	return n, err
}

func (f *storageFile) Write(b []byte) (int, error) {
	n, err := f.fs.Write(b)
	return n, err
}

func (f *storageFile) Close() error {
	if f.fs == nil {
		return nil
	}

	err := f.fs.Close()
	if err == nil {
		f.fs = nil
	}

	return err
}

func (f *storageFile) Remove() error {
	return os.Remove(f.filepath)
}
