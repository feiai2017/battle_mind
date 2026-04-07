package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupFileLogging_WritesToFile(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "logs", "server.log")

	originalWriter := log.Writer()
	originalFlags := log.Flags()
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	log.SetFlags(0)
	closer, err := SetupFileLogging(logPath)
	if err != nil {
		t.Fatalf("setup logging failed: %v", err)
	}
	if closer == nil {
		t.Fatal("expected closer")
	}
	defer closer.Close()

	log.Print("component=test event=hello")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if !strings.Contains(string(data), "component=test event=hello") {
		t.Fatalf("unexpected log file contents: %s", string(data))
	}
}
