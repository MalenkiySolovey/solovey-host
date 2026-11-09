//go:build !minimal

package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

const testBotToken = "123456789:abcdefghijklmnopqrstuvwxyz012345678"

func TestBotClientUsesInjectedTransportAndRedactsErrors(t *testing.T) {
	calls := 0
	client := NewBotClient(testBotToken, &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Host != "api.telegram.org" || request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/json" {
			t.Fatal("request lost its bounded caller transport or API binding")
		}
		return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"ok":false,"error_code":429,"description":"token ` + testBotToken + `","parameters":{"retry_after":7}}`))}, nil
	})})
	_, err := client.CallJSON(t.Context(), "sendMessage", map[string]string{"text": "fixture"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != 429 || apiErr.RetryAfter != 7 || strings.Contains(err.Error(), testBotToken) || calls != 1 {
		t.Fatal("Bot API status, retry or redaction changed")
	}
	if caption := Caption(strings.Repeat("界", 1000) + "\r\n\x00"); len(caption) > 1024 || strings.ContainsAny(caption, "\r\n\x00") {
		t.Fatal("caption escaped its UTF-8/control bound")
	}
}

func TestBotClientRejectsOversizedResponsesAndUnsafeRequests(t *testing.T) {
	calls := 0
	client := NewBotClient(testBotToken, &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", maxTelegramResponseBytes+1)))}, nil
	})})
	if _, err := client.DoJSON(t.Context(), "sendMessage", struct{}{}); err != ErrRequest {
		t.Fatal("oversized API response escaped")
	}
	for _, method := range []string{"../sendMessage", "sendMessage?token=other", "https://other.test", ""} {
		if _, err := client.CallJSON(t.Context(), method, struct{}{}); err != ErrRequest {
			t.Fatal("untrusted API method reached the transport")
		}
	}
	if calls != 1 {
		t.Fatal("invalid request performed network work")
	}
	if _, err := NewBotClient(testBotToken, nil).CallJSON(t.Context(), "sendMessage", struct{}{}); err != ErrNetwork {
		t.Fatal("missing transport silently selected a client")
	}
}

func TestBotClientCancellationHidesTransportErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	client := NewBotClient(testBotToken, &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, errors.New("private transport detail " + testBotToken)
	})})
	if _, err := client.CallGET(ctx, "getMe", nil); err != ErrNetwork || ctx.Err() == nil {
		t.Fatal("cancellation or transport redaction changed")
	}
}
