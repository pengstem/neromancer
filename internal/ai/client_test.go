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

	"github.com/pengstem/neromancer/internal/config"
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

type chatTestTransport func(*http.Request) (*http.Response, error)

func (f chatTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGenerateActionPlanMarkdownAndTruncation(t *testing.T) {
	const plan = "### 工具\n\n- 试一下 `print(\"hello\")`。\n\n### 第一步\n\n- 写一个小例子。\n\n### 第二步\n\n- 整理目录。\n\n### 第三步\n\n- 找朋友试用一次。"
	type reply struct {
		reason  string
		content string
	}
	complete := reply{reason: "stop", content: "\n" + plan + "\n"}
	truncated := reply{reason: "length", content: "### 工具\n\n- 还没写"}
	for _, tc := range []struct {
		name    string
		replies []reply
		want    string
		wantErr string
	}{
		{name: "complete", replies: []reply{complete}, want: plan},
		{name: "retry concise", replies: []reply{truncated, complete}, want: plan},
		{name: "retry also truncated", replies: []reply{truncated, truncated}, wantErr: errChatOutputTruncated.Error()},
		{name: "empty", replies: []reply{{reason: "stop", content: " \n "}}, wantErr: "AI 这次返回了空内容"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient("https://api.example.com", "sk-test", "deepseek-chat")
			calls := 0
			c.http.Transport = chatTestTransport(func(r *http.Request) (*http.Response, error) {
				if calls >= len(tc.replies) {
					return nil, errors.New("unexpected extra request")
				}
				var req chatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Fatalf("decode plan request: %v", err)
				}
				if r.URL.Path != "/v1/chat/completions" || req.ResponseFormat != nil {
					t.Fatalf("expected Markdown chat request, got path=%q format=%v", r.URL.Path, req.ResponseFormat)
				}
				wantPrompt := actionPlanSystemPrompt
				if calls == 1 {
					wantPrompt += actionPlanCompactPrompt
				}
				if req.Messages[0].Content != wantPrompt || !strings.Contains(req.Messages[1].Content.(string), "做个小工具") {
					t.Fatalf("unexpected messages in request %d: %+v", calls+1, req.Messages)
				}
				rpl := tc.replies[calls]
				calls++
				w := httptest.NewRecorder()
				_, _ = fmt.Fprintf(w, `{"choices":[{"finish_reason":%q,"message":{"content":%q}}]}`, rpl.reason, rpl.content)
				return w.Result(), nil
			})

			got, err := c.GenerateActionPlan(context.Background(), domain.IdeaNode{Title: "做个小工具"}, nil)
			if got != tc.want || calls != len(tc.replies) {
				t.Fatalf("got plan %q in %d calls, want %q in %d calls (error: %v)", got, calls, tc.want, len(tc.replies), err)
			}
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("got error %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestChatJSONRejectsTruncatedChoiceBeforeParsing(t *testing.T) {
	c := NewClient("https://api.example.com", "sk-test", "deepseek-chat")
	c.http.Transport = chatTestTransport(func(r *http.Request) (*http.Response, error) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode JSON mode request: %v", err)
		}
		if req.ResponseFormat["type"] != "json_object" {
			t.Fatalf("expected JSON mode, got %v", req.ResponseFormat)
		}
		w := httptest.NewRecorder()
		_, _ = w.WriteString(`{"choices":[{"finish_reason":"length","message":{"content":"{\"spark\":\"还没写"}}]}`)
		return w.Result(), nil
	})
	if _, err := c.SuggestFromTags(context.Background(), nil, nil); !errors.Is(err, errChatOutputTruncated) {
		t.Fatalf("expected truncation error, got %v", err)
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

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://api.moark.com/v1/chat/completions", "https://api.moark.com"},
		{"https://api.moark.com/v1/chat/completions/", "https://api.moark.com"},
		{"https://api.moark.com/v1/", "https://api.moark.com"},
		{"https://api.moark.com", "https://api.moark.com"},
		{"  ", config.DefaultDeepSeekBaseURL},
	}
	for _, tc := range cases {
		if got := normalizeBaseURL(tc.in); got != tc.want {
			t.Fatalf("normalizeBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTranscribeAudioFallbackToOpenAITranscriptions(t *testing.T) {
	var paths []string
	var gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/v1/tools/asr" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path != "/v1/audio/transcriptions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotModel = r.FormValue("model")
		file, hdr, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("missing form file: %v", err)
		}
		_ = file.Close()
		if !strings.HasSuffix(hdr.Filename, ".wav") {
			t.Fatalf("expected wav filename, got %s", hdr.Filename)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"模力方舟语音转写成功","language":"auto"}`))
	}))
	defer srv.Close()

	wavPath := filepath.Join(t.TempDir(), "spark.wav")
	if err := os.WriteFile(wavPath, []byte("RIFF\x00\x00\x00\x00WAVEfmt "), 0o644); err != nil {
		t.Fatal(err)
	}

	c := NewClient(srv.URL, "sk-test-moark", "deepseek-v4.1-flash")
	res, err := c.TranscribeAudio(context.Background(), wavPath)
	if err != nil {
		t.Fatalf("TranscribeAudio fallback failed: %v", err)
	}
	if len(paths) != 2 || paths[0] != "/v1/tools/asr" || paths[1] != "/v1/audio/transcriptions" {
		t.Fatalf("unexpected call sequence: %v", paths)
	}
	if gotModel != defaultOpenAIASRModel {
		t.Fatalf("expected model %q, got %q", defaultOpenAIASRModel, gotModel)
	}
	if res.Text != "模力方舟语音转写成功" {
		t.Fatalf("unexpected ASR text: %+v", res)
	}
}
