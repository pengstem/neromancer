package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pengstem/neromancer/internal/domain"
)

func TestClientRequiresAPIKey(t *testing.T) {
	c := NewClient("https://api.deepseek.com", "", "deepseek-flash")
	if c.Configured() {
		t.Fatalf("expected Configured() to be false without API key")
	}

	_, err := c.TestConnection(context.Background())
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("expected ErrMissingAPIKey, got %v", err)
	}
}

func TestIngestAndOrganizeMultimodalAndJSONMode(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var sawImageBlock bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode chat request: %v", err)
		}
		msgs, _ := payload["messages"].([]any)
		if len(msgs) >= 2 {
			userMsg, _ := msgs[1].(map[string]any)
			if blocks, ok := userMsg["content"].([]any); ok && len(blocks) == 2 {
				sawImageBlock = true
			}
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"finish_reason": "stop",
					"message": map[string]any{
						"content": "```json\n{\"action\":\"merge\",\"target_node_id\":\"orb_tulip\",\"title\":\"用一个蓝色花瓶养一株郁金香\",\"conversational_summary\":\"把花店灵光和蓝色花瓶拧成一件事。\",\"tags\":[\"郁金香\",\"蓝色花瓶\"],\"theme\":\"coral\",\"weight\":0.95,\"visual\":{\"motif_type\":\"petals\",\"accent_hex\":\"#3B82F6\",\"caption\":\"蓝玻璃衬粉花\",\"energy\":0.9},\"guiding_questions\":[\"要买单支还是三支？\"]}\n```",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	// Create tiny PNG fixture to exercise multimodal image_url encoding.
	imgPath := filepath.Join(t.TempDir(), "vase.png")
	if err := os.WriteFile(imgPath, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient(srv.URL, "sk-test-deepseek", "deepseek-flash")
	decision, err := c.IngestAndOrganize(context.Background(), domain.RawEvidence{
		ID:         "ev_test",
		Modality:   "image",
		RawText:    "郁金香的花瓶应该是蓝色的",
		ImagePath:  imgPath,
		CapturedAt: time.Now(),
	}, []NodeDigest{{ID: "orb_tulip", Title: "买一盆郁金香放在阳台上养"}})
	if err != nil {
		t.Fatalf("IngestAndOrganize failed: %v", err)
	}
	if capturedPath != "/v1/chat/completions" {
		t.Fatalf("unexpected endpoint path: %s", capturedPath)
	}
	if capturedAuth != "Bearer sk-test-deepseek" {
		t.Fatalf("unexpected auth header: %s", capturedAuth)
	}
	if !sawImageBlock {
		t.Fatalf("expected multimodal image_url block in user content")
	}
	if decision.Action != "merge" || decision.TargetNodeID != "orb_tulip" {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}

func TestTranscribeAudioDeepSeekASR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tools/asr" {
			t.Fatalf("expected /v1/tools/asr, got %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		file, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing form file: %v", err)
		}
		_ = file.Close()
		if !strings.HasSuffix(hdr.Filename, ".wav") {
			t.Fatalf("expected wav filename, got %s", hdr.Filename)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"request_id":"req_123","text":"郁金香的花瓶应该是蓝色的","duration_ms":2400}`))
	}))
	defer srv.Close()

	wavPath := filepath.Join(t.TempDir(), "spark.wav")
	if err := os.WriteFile(wavPath, []byte("RIFF\x00\x00\x00\x00WAVEfmt "), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient(srv.URL, "sk-test-asr", "deepseek-flash")
	res, err := c.TranscribeAudio(context.Background(), wavPath)
	if err != nil {
		t.Fatalf("TranscribeAudio failed: %v", err)
	}
	if res.Text != "郁金香的花瓶应该是蓝色的" || res.DurationMs != 2400 {
		t.Fatalf("unexpected ASR response: %+v", res)
	}
}

func TestIngestAndOrganizeReusesExistingTagsAndStripsHashes(t *testing.T) {
	var userPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if len(payload.Messages) >= 2 {
			userPrompt = payload.Messages[1].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"create\",\"title\":\"新笔记\",\"conversational_summary\":\"摘要\",\"tags\":[\"#生活器物\",\"＃阳台植物\",\"生活器物\",\"\"],\"theme\":\"mint\",\"weight\":0.8}"}}]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "sk-test", "deepseek-flash")
	decision, err := c.IngestAndOrganize(context.Background(), domain.RawEvidence{
		ID:         "ev_hash",
		Modality:   "text",
		RawText:    "换个陶土盆透气更好",
		CapturedAt: time.Now(),
	}, []NodeDigest{
		{ID: "n1", Title: "花瓶", Tags: []string{"#生活器物", "阳台植物"}},
		{ID: "n2", Title: "咖啡杯", Tags: []string{"生活器物"}},
	})
	if err != nil {
		t.Fatalf("IngestAndOrganize: %v", err)
	}
	if !strings.Contains(userPrompt, "生活器物、阳台植物") {
		t.Fatalf("expected existing high-frequency tags injected into user prompt, got: %s", userPrompt)
	}
	if len(decision.Tags) != 2 || decision.Tags[0] != "生活器物" || decision.Tags[1] != "阳台植物" {
		t.Fatalf("expected sanitized deduplicated tags without '#', got %+v", decision.Tags)
	}
}

func TestIsConfigUnusableError(t *testing.T) {
	if IsConfigUnusableError(nil) {
		t.Fatalf("expected nil error to be false")
	}
	if !IsConfigUnusableError(ErrMissingAPIKey) {
		t.Fatalf("expected ErrMissingAPIKey to be unusable")
	}
	for _, status := range []int{http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden, http.StatusNotFound} {
		err := &APIError{StatusCode: status, Message: "bad config"}
		if !IsConfigUnusableError(err) {
			t.Fatalf("expected HTTP %d APIError to be unusable", status)
		}
		wrapped := fmt.Errorf("request failed: %w", err)
		if !IsConfigUnusableError(wrapped) {
			t.Fatalf("expected wrapped HTTP %d APIError to be unusable", status)
		}
	}
	for _, status := range []int{http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway} {
		err := &APIError{StatusCode: status, Message: "transient"}
		if IsConfigUnusableError(err) {
			t.Fatalf("expected HTTP %d transient APIError NOT to be treated as unusable config", status)
		}
	}
	if IsConfigUnusableError(errors.New("dial tcp: timeout")) {
		t.Fatalf("expected plain network timeout NOT to be treated as unusable config")
	}
}
