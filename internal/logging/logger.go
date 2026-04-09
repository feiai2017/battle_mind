package logging

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// SetupFileLogging configures the standard logger to write to stdout and a file.
func SetupFileLogging(filePath string) (io.Closer, error) {
	path := strings.TrimSpace(filePath)
	if path == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(io.MultiWriter(os.Stdout, file))
	return file, nil
}
