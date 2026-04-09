package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestLogTextBlock_WritesReadableBlock(t *testing.T) {
	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	log.SetOutput(&buf)
	log.SetFlags(0)

	LogTextBlock("analyze_service", "prompt", "req-1", "line1\nline2")

	output := buf.String()
	for _, want := range []string{
		"----- [ANALYZE_SERVICE][PROMPT] BEGIN request_id=req-1",
		"[ANALYZE_SERVICE][PROMPT][001] line1",
		"[ANALYZE_SERVICE][PROMPT][002] line2",
		"----- [ANALYZE_SERVICE][PROMPT] END request_id=req-1 -----",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("missing %q in output: %s", want, output)
		}
	}
}

func TestLogJSONBlock_WritesIndentedJSON(t *testing.T) {
	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	defer func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	}()

	log.SetOutput(&buf)
	log.SetFlags(0)

	LogJSONBlock("handler", "response", "req-2", map[string]any{"ok": true})

	output := buf.String()
	if !strings.Contains(output, "\"ok\": true") {
		t.Fatalf("expected indented json in output: %s", output)
	}
}
