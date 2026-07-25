package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage stores files on the local filesystem and serves them via the
// static middleware at /uploads/<key>.
type LocalStorage struct {
	uploadDir  string
	serverBase string // e.g. "http://localhost:3000"
}

func newLocal(uploadDir, serverBase string) *LocalStorage {
	return &LocalStorage{uploadDir: uploadDir, serverBase: serverBase}
}

func (l *LocalStorage) Upload(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	dest := filepath.Join(l.uploadDir, key)
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, body)
	return err
}

func (l *LocalStorage) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(l.uploadDir, key))
}

func (l *LocalStorage) PublicURL(key string) string {
	return l.serverBase + "/uploads/" + key
}

func (l *LocalStorage) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return l.PublicURL(key), nil
}

func (l *LocalStorage) Size(_ context.Context, key string) (int64, bool, error) {
	info, err := os.Stat(filepath.Join(l.uploadDir, key))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return info.Size(), true, nil
}
