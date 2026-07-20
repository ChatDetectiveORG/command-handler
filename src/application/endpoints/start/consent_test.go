package start

import (
	"strings"
	"testing"

	constants "github.com/ChatDetectiveORG/shared/constants"
	"github.com/ChatDetectiveORG/shared/legal"
	tele "gopkg.in/telebot.v4"
)

func testDocs() legal.Docs {
	return legal.Docs{
		AgreementURL: "https://example.com/agreement",
		PrivacyURL:   "https://example.com/privacy",
		ConsentURL:   "https://example.com/consent",
		Version:      "2026-07-20",
	}
}

func TestBuildConsentMessageLinksAndButton(t *testing.T) {
	msg := buildConsentMessage(testDocs(), 42)

	if msg.Chat.ID != 42 {
		t.Fatalf("unexpected chat id: %d", msg.Chat.ID)
	}
	if !strings.Contains(msg.Text, "версия 2026-07-20") {
		t.Fatalf("expected docs version in text, got: %s", msg.Text)
	}

	var links []string
	for _, entity := range msg.Entities {
		if entity.Type == tele.EntityTextLink {
			links = append(links, entity.URL)
		}
	}
	expected := []string{
		"https://example.com/agreement",
		"https://example.com/privacy",
		"https://example.com/consent",
	}
	if len(links) != len(expected) {
		t.Fatalf("expected %d links, got %d", len(expected), len(links))
	}
	for i, url := range expected {
		if links[i] != url {
			t.Fatalf("link %d: expected %s, got %s", i, url, links[i])
		}
	}

	keyboard := msg.ReplyMarkup.InlineKeyboard
	if len(keyboard) != 1 || len(keyboard[0]) != 1 {
		t.Fatalf("expected single accept button, got %+v", keyboard)
	}
	if keyboard[0][0].Data != constants.UniqueLegalConsent {
		t.Fatalf("unexpected button data: %s", keyboard[0][0].Data)
	}
}

func TestNeedsConsentGateSkippedWhenDocsNotConfigured(t *testing.T) {
	needs, err := needsConsentGate(nil, legal.Docs{})
	if !err.IsNil() {
		t.Fatalf("unexpected error: %s", err.JSON())
	}
	if needs {
		t.Fatal("gate must be disabled when legal docs are not configured")
	}
}
