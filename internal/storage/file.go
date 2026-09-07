package storage

import (
	"errors"
	"fmt"
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
	file     *os.File
	filepath string
}

func NewStorageFile(s *Storage, storagePath string) (*storageFile, error) {
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
			log.Warn("Is not dir", "dir", dir, "filepath", filepath)
			err := os.Remove(dir)
			if err != nil {
				return nil, fmt.Errorf("Remove dir %s: %w", filepath, err)
			}
			err = os.MkdirAll(dir, 0775)
			if err != nil {
				return nil, fmt.Errorf("Mkdir %s: %w", filepath, err)
			}
		}
	}

	flag := os.O_RDONLY | os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	fs, err := os.OpenFile(filepath, flag, 0666)
	if err != nil {
		return nil, fmt.Errorf("Open file %s: %w", filepath, err)
	}

	return &storageFile{
		filepath: filepath,
		file:     fs,
	}, nil
}

func (f *storageFile) Read(b []byte) (int, error) {
	n, err := f.file.Read(b)
	return n, err
}

func (f *storageFile) Write(b []byte) (int, error) {
	n, err := f.file.Write(b)
	return n, err
}

func (f *storageFile) Close() error {
	if f.file == nil {
		return nil
	}

	err := f.file.Close()
	if err == nil {
		f.file = nil
	}

	return err
}

func (f *storageFile) Remove() error {
	err := os.Remove(f.filepath)

	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func (f *storageFile) log(name string) *slog.Logger {
	op := "StorageFile." + name
	return slog.Default().With(slog.String("op", op))
}
