//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

type mailhogListResponse struct {
	Items []mailhogMessage `json:"items"`
}

type mailhogMessage struct {
	Content struct {
		Headers map[string][]string `json:"Headers"`
		Body    string              `json:"Body"`
	} `json:"Content"`
}

func mailhogMessagesURL() string {
	if value := os.Getenv("TEST_MAILHOG_URL"); value != "" {
		return value
	}
	return "http://localhost:8025/api/v2/messages"
}

func waitForMailhogBody(t *testing.T, toEmail, subject string) string {
	t.Helper()

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(mailhogMessagesURL())
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		var list mailhogListResponse
		if err := json.Unmarshal(raw, &list); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for i := len(list.Items) - 1; i >= 0; i-- {
			item := list.Items[i]
			if !headerContains(item.Content.Headers["To"], toEmail) {
				continue
			}
			if !headerContains(item.Content.Headers["Subject"], subject) {
				continue
			}
			if strings.TrimSpace(item.Content.Body) != "" {
				return normalizeMailhogBody(item.Content.Body)
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for Mailhog message to %s with subject %q", toEmail, subject)
	return ""
}

func normalizeMailhogBody(body string) string {
	decoded, err := io.ReadAll(quotedprintable.NewReader(bytes.NewBufferString(body)))
	if err == nil && len(decoded) > 0 {
		return string(decoded)
	}

	// Fallback for soft line breaks if the full quoted-printable decode fails.
	return strings.ReplaceAll(body, "=\r\n", "")
}

func headerContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func extractTokenFromMailBody(t *testing.T, body, routePrefix string) string {
	t.Helper()

	pattern := regexp.MustCompile(regexp.QuoteMeta(routePrefix) + `([^"'<>\\s]+)`)
	match := pattern.FindStringSubmatch(body)
	if len(match) < 2 {
		t.Fatalf("could not extract token for prefix %q from mail body: %s", routePrefix, body)
	}
	return match[1]
}

func requireBodyContains(t *testing.T, body string, expected ...string) {
	t.Helper()

	for _, fragment := range expected {
		if !strings.Contains(body, fragment) {
			t.Fatalf("expected mail body to contain %q, got: %s", fragment, body)
		}
	}
}

func dumpHeaders(headers map[string][]string) string {
	return fmt.Sprintf("%v", headers)
}
