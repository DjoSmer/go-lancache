package lancache

import (
	"net/http"

	"go-lancache/internal/storage"
)

type LancacheResponseWriter struct {
	writer      http.ResponseWriter
	storageFile storage.StorageFile
	storageErr  error
}

func NewLancacheResponseWriter(storageFile storage.StorageFile, writer http.ResponseWriter) *LancacheResponseWriter {
	return &LancacheResponseWriter{
		writer:      writer,
		storageFile: storageFile,
	}
}

func (w *LancacheResponseWriter) GetStorageError() error {
	return w.storageErr
}

func (w *LancacheResponseWriter) Flush() {
	if f, ok := w.writer.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *LancacheResponseWriter) Header() http.Header {
	return w.writer.Header()
}

func (w *LancacheResponseWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	if err != nil {
		return n, err
	}

	if n > 0 && w.storageErr == nil {
		_, storageErr := w.storageFile.Write(p[:n])
		if storageErr != nil {
			w.storageErr = storageErr
		}
	}

	return n, err
}

func (w *LancacheResponseWriter) WriteHeader(statusCode int) {
	w.writer.WriteHeader(statusCode)
}
