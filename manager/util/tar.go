package util

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Tar(src string) (io.Reader, error) {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil, fmt.Errorf("path does not exist: %s", src)
	}

	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	pathLen := len(src)

	err := filepath.Walk(src, func(file string, info os.FileInfo, lerr error) error {
		header, err := tar.FileInfoHeader(info, file)
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(file)[pathLen:]

		if err := writer.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			data, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(writer, data); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &buffer, nil
}
