package services

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ResolveTemplate replaces placeholders in a destination table name template.
// Supported placeholders: {{TABLE}}, {{UUID}}, {{YEAR}}, {{MONTH}}, {{DAY}}, {{HOUR}}, {{MINUTE}}.
func ResolveTemplate(template, tableName, cdcID string) (string, error) {
	now := time.Now().UTC()

	result := template
	result = strings.ReplaceAll(result, "{{TABLE}}", tableName)
	result = strings.ReplaceAll(result, "{{YEAR}}", now.Format("2006"))
	result = strings.ReplaceAll(result, "{{MONTH}}", now.Format("01"))
	result = strings.ReplaceAll(result, "{{DAY}}", now.Format("02"))
	result = strings.ReplaceAll(result, "{{HOUR}}", now.Format("15"))
	result = strings.ReplaceAll(result, "{{MINUTE}}", now.Format("04"))

	if strings.Contains(result, "{{UUID}}") {
		u := uuid.New().String()
		result = strings.ReplaceAll(result, "{{UUID}}", u)
	}

	return result, nil
}
