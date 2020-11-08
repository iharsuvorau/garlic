package main

import (
	"io"
	"os"
	"path"
)

type FileStore struct {
	base string
}

func NewFileStore(base string) *FileStore {
	return &FileStore{
		base: base,
	}
}

func (s *FileStore) Save(name string, src io.Reader) (string, error) {
	dst := path.Join(s.base, name)

	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err = io.Copy(f, src); err != nil {
		return "", err
	}

	return dst, nil
}

func (s *FileStore) Get(name string) (*os.File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *FileStore) Delete(filename string) error {
	return removeFile(filename)
}
