package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
)

func LogFields(component, category, requestID string, fields map[string]any) {
	component = normalizeLabel(component)
	category = normalizeLabel(category)
	requestID = strings.TrimSpace(requestID)

	log.Printf("----- [%s][%s] BEGIN request_id=%s -----", component, category, requestID)

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		log.Printf("[%s][%s] %-18s %v", component, category, key+":", fields[key])
	}

	log.Printf("----- [%s][%s] END request_id=%s -----", component, category, requestID)
}

func LogTextBlock(component, category, requestID, content string) {
	component = normalizeLabel(component)
	category = normalizeLabel(category)
	requestID = strings.TrimSpace(requestID)
	content = normalizeNewlines(content)

	lines := strings.Split(content, "\n")
	if content == "" {
		lines = []string{"(empty)"}
	}

	log.Printf(
		"----- [%s][%s] BEGIN request_id=%s lines=%d chars=%d -----",
		component,
		category,
		requestID,
		len(lines),
		len(content),
	)
	for i, line := range lines {
		log.Printf("[%s][%s][%03d] %s", component, category, i+1, line)
	}
	log.Printf("----- [%s][%s] END request_id=%s -----", component, category, requestID)
}

func LogJSONBlock(component, category, requestID string, value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		LogTextBlock(component, category+"_MARSHAL_ERROR", requestID, fmt.Sprintf("marshal failed: %v", err))
		return
	}
	LogTextBlock(component, category, requestID, string(data))
}

func normalizeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "UNKNOWN"
	}
	return strings.ToUpper(value)
}

func normalizeNewlines(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n")
}
