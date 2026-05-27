package subscription

import (
	"bytes"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type failingSubscriptionService struct{}

func (f failingSubscriptionService) GenerateByToken(token string) (string, error) {
	return "", errors.New("upstream returned status 403")
}

func (f failingSubscriptionService) GenerateByTokenWithRelayHost(token, relayHost string) (string, error) {
	return f.GenerateByToken(token)
}

func TestServeTokenLogsGenerationError(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousOutput)

	handler := Handler{Service: failingSubscriptionService{}}
	req := httptest.NewRequest(http.MethodGet, "/sub/token-secret-value", nil)
	res := httptest.NewRecorder()

	handler.ServeToken(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusBadGateway)
	}
	if !strings.Contains(logs.String(), "subscription generation failed") || !strings.Contains(logs.String(), "upstream returned status 403") {
		t.Fatalf("expected detailed generation error in logs, got %q", logs.String())
	}
	if strings.Contains(logs.String(), "token-secret-value") {
		t.Fatalf("log leaked full token: %q", logs.String())
	}
}
