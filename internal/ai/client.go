// Package ai implements the unified DeepSeek API client for Neromancer.
// It uses real DeepSeek endpoints (/v1/chat/completions with multimodal image_url support
// and /v1/tools/asr for speech-to-text) and never fabricates mock AI outputs.
package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pengstem/neromancer/internal/config"
	"github.com/pengstem/neromancer/internal/domain"
)

const maxASRBytes = 8 * 1024 * 1024 // DeepSeek /v1/tools/asr max 8 MiB

var (
	// ErrMissingAPIKey indicates the user has not configured a DeepSeek API key yet.
	ErrMissingAPIKey = errors.New("尚未配置 DeepSeek API Key，请先在右上角「⚙️ DeepSeek 设置」中填入")
)

// NodeDigest gives the organizer compact context of existing orbs without inflating token usage.
type NodeDigest struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Summary string   `json:"summary"`
	Tags    []string `json:"tags"`
	Stage   string   `json:"stage"`
}

// OrganizeDecision represents DeepSeek's structured decision to merge or create an IdeaNode.
type OrganizeDecision struct {
	Action                string             `json:"action"` // "merge" | "create"
	TargetNodeID          string             `json:"target_node_id"`
	Title                 string             `json:"title"`
	ConversationalSummary string             `json:"conversational_summary"`
	Tags                  []string           `json:"tags"`
	Theme                 string             `json:"theme"`
	Weight                float64            `json:"weight"`
	Visual                domain.VisualMotif `json:"visual"`
	GuidingQuestions      []string           `json:"guiding_questions"`
	RelatedIDs            []string           `json:"related_ids"`
}

// ASRResult matches DeepSeek POST /v1/tools/asr response payload.
type ASRResult struct {
	RequestID  string `json:"request_id"`
	Text       string `json:"text"`
	DurationMs int    `json:"duration_ms"`
}

// CollisionResult holds a cross-pollination output from combining user + roam inspirations.
type CollisionResult struct {
	Title                 string   `json:"title"`
	ConversationalSummary string   `json:"conversational_summary"`
	FirstStep             string   `json:"first_step"`
	Tags                  []string `json:"tags"`
	Theme                 string   `json:"theme"`
}

// Client talks directly to DeepSeek's HTTP APIs.
type Client struct {
	mu      sync.RWMutex
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

// NewClient constructs a DeepSeek API client from runtime settings.
func NewClient(baseURL, apiKey, model string) *Client {
	return &Client{
		baseURL: normalizeBaseURL(baseURL),
		apiKey:  strings.TrimSpace(apiKey),
		model:   defaultString(model, config.DefaultDeepSeekModel),
		http:    &http.Client{Timeout: 75 * time.Second},
	}
}

// UpdateCredentials updates DeepSeek credentials at runtime when saved from Settings UI.
func (c *Client) UpdateCredentials(baseURL, apiKey, model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = normalizeBaseURL(baseURL)
	c.apiKey = strings.TrimSpace(apiKey)
	c.model = defaultString(model, config.DefaultDeepSeekModel)
}

// Snapshot returns current baseURL, apiKey presence, and model.
func (c *Client) Snapshot() (baseURL, apiKey, model string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.baseURL, c.apiKey, c.model
}

// Configured reports whether a non-empty DeepSeek API key is set.
func (c *Client) Configured() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey != ""
}

// TestConnection queries GET /v1/models on DeepSeek to verify API key validity and available models.
func (c *Client) TestConnection(ctx context.Context) (string, error) {
	baseURL, apiKey, model, err := c.requireAuth()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接 DeepSeek 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if resp.StatusCode >= 400 {
		return "", formatAPIError(resp.StatusCode, body)
	}

	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 模型列表失败: %w", err)
	}

	var ids []string
	for _, m := range list.Data {
		ids = append(ids, m.ID)
	}
	return fmt.Sprintf("连接成功！当前选用模型：%s（可用模型：%s）", model, strings.Join(ids, ", ")), nil
}

// TranscribeAudio calls POST /v1/tools/asr with a 16-bit PCM WAV or Ogg Opus file (<= 8 MiB).
func (c *Client) TranscribeAudio(ctx context.Context, audioPath string) (*ASRResult, error) {
	baseURL, apiKey, _, err := c.requireAuth()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(audioPath)
	if err != nil {
		return nil, fmt.Errorf("读取音频文件失败: %w", err)
	}
	if info.Size() > maxASRBytes {
		return nil, fmt.Errorf("音频文件超过 DeepSeek ASR 8 MiB 限制（当前 %.2f MiB）", float64(info.Size())/(1024*1024))
	}

	f, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/tools/asr", &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 DeepSeek ASR 失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if resp.StatusCode >= 400 {
		return nil, formatAPIError(resp.StatusCode, body)
	}

	var out ASRResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析 DeepSeek ASR 响应失败: %w", err)
	}
	out.Text = strings.TrimSpace(out.Text)
	return &out, nil
}

// IngestAndOrganize asks DeepSeek to deduplicate/merge or create an organized IdeaNode from a RawEvidence.
// When ev.ImagePath points to an image, it passes an inline Base64 image_url multimodal block to deepseek-flash.
func (c *Client) IngestAndOrganize(ctx context.Context, ev domain.RawEvidence, catalog []NodeDigest) (*OrganizeDecision, error) {
	catalogJSON, _ := json.Marshal(catalog)
	promptText := fmt.Sprintf(
		"【新捕捉到的原始证据】\n- 模态：%s\n- 记录时间：%s\n- 场景：%s\n- 原文内容：%s\n\n【当前已有灵感目录摘要】\n%s",
		ev.Modality,
		ev.CapturedAt.Format("2006-01-02 15:04"),
		ev.Scene.SummaryLine(),
		defaultString(ev.RawText, "（用户上传了一张图片/涂鸦，请直接理解图像内容）"),
		string(catalogJSON),
	)

	userContent, err := buildMultimodalContent(promptText, ev.ImagePath)
	if err != nil {
		return nil, err
	}

	raw, err := c.chatJSON(ctx, organizeSystemPrompt, userContent)
	if err != nil {
		return nil, err
	}

	var decision OrganizeDecision
	if err := decodeJSONFlexible(raw, &decision); err != nil {
		return nil, fmt.Errorf("解析 DeepSeek 组织决策 JSON 失败: %w", err)
	}
	if decision.Title == "" {
		decision.Title = summarizeFallbackTitle(ev.RawText)
	}
	if decision.Weight <= 0 {
		decision.Weight = 0.72
	}
	return &decision, nil
}

// GenerateGuidingQuestions generates 3 sharp, non-generic exploratory questions.
func (c *Client) GenerateGuidingQuestions(ctx context.Context, node domain.IdeaNode, evidences []domain.RawEvidence) ([]string, error) {
	payload := buildNodeContextPrompt(node, evidences)
	raw, err := c.chatJSON(ctx, guidingQuestionsSystemPrompt, payload)
	if err != nil {
		return nil, err
	}
	var out struct {
		Questions []string `json:"questions"`
	}
	if err := decodeJSONFlexible(raw, &out); err != nil {
		return nil, fmt.Errorf("解析追问 JSON 失败: %w", err)
	}
	return out.Questions, nil
}

// GenerateActionPlan produces a step-by-step actionable Markdown roadmap ("AI 帮忙").
func (c *Client) GenerateActionPlan(ctx context.Context, node domain.IdeaNode, evidences []domain.RawEvidence) (string, error) {
	payload := buildNodeContextPrompt(node, evidences)
	raw, err := c.chatJSON(ctx, actionPlanSystemPrompt, payload)
	if err != nil {
		return "", err
	}
	var out struct {
		ActionPlanMD string `json:"action_plan_md"`
	}
	if err := decodeJSONFlexible(raw, &out); err != nil {
		return "", fmt.Errorf("解析可执行方案 JSON 失败: %w", err)
	}
	return strings.TrimSpace(out.ActionPlanMD), nil
}

// SuggestFromTags generates a light new inspiration prompt from existing tags on Showcase view.
func (c *Client) SuggestFromTags(ctx context.Context, tags []string, recentTitles []string) (string, error) {
	userPrompt := fmt.Sprintf("已有活跃标签：%s\n最近记忆球：%s\n请给一条轻巧有趣的新灵感提醒。",
		strings.Join(tags, "、"), strings.Join(recentTitles, " / "))
	raw, err := c.chatJSON(ctx, tagSparkSystemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	var out struct {
		Spark string `json:"spark"`
	}
	if err := decodeJSONFlexible(raw, &out); err != nil {
		return "", fmt.Errorf("解析新灵感提示 JSON 失败: %w", err)
	}
	return strings.TrimSpace(out.Spark), nil
}

// CollideInspirations fuses one of the user's orbs with another creator's RoamSlice.
func (c *Client) CollideInspirations(ctx context.Context, mine domain.IdeaNode, roam domain.RoamSlice) (*CollisionResult, error) {
	userPrompt := fmt.Sprintf(
		"【我的记忆球】\n标题：%s\n摘要：%s\n标签：%s\n\n【他人的灵感切片（来自 %s）】\n标题：%s\n内容：%s\n场景：%s\n\n请碰撞出一个新的灵感！",
		mine.Title, mine.ConversationalSummary, strings.Join(mine.Tags, "、"),
		roam.AuthorAlias, roam.Title, roam.Snippet, roam.SceneHint,
	)
	raw, err := c.chatJSON(ctx, collideSystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	var out CollisionResult
	if err := decodeJSONFlexible(raw, &out); err != nil {
		return nil, fmt.Errorf("解析灵感碰撞 JSON 失败: %w", err)
	}
	return &out, nil
}

// SleepIncubate runs the background/overnight connection-finder across the whole vault.
func (c *Client) SleepIncubate(ctx context.Context, nodes []domain.IdeaNode) ([]domain.SleepInsight, error) {
	var lines []string
	for _, n := range nodes {
		lines = append(lines, fmt.Sprintf("- id=%s | title=%s | tags=%s | summary=%s",
			n.ID, n.Title, strings.Join(n.Tags, ","), n.ConversationalSummary))
	}
	userPrompt := "【用户全部灵感库】\n" + strings.Join(lines, "\n")
	raw, err := c.chatJSON(ctx, sleepIncubateSystemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}
	var out struct {
		Insights []struct {
			Title            string   `json:"title"`
			ConnectedNodeIDs []string `json:"connected_node_ids"`
			SparkSummary     string   `json:"spark_summary"`
			SuggestedAction  string   `json:"suggested_action"`
		} `json:"insights"`
	}
	if err := decodeJSONFlexible(raw, &out); err != nil {
		return nil, fmt.Errorf("解析睡眠孵化 JSON 失败: %w", err)
	}
	var res []domain.SleepInsight
	for i, item := range out.Insights {
		res = append(res, domain.SleepInsight{
			ID:               fmt.Sprintf("sleep_%d_%d", time.Now().UnixNano(), i),
			Title:            item.Title,
			ConnectedNodeIDs: item.ConnectedNodeIDs,
			SparkSummary:     item.SparkSummary,
			SuggestedAction:  item.SuggestedAction,
			CreatedAt:        time.Now(),
		})
	}
	return res, nil
}

type chatRequest struct {
	Model          string         `json:"model"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat map[string]any `json:"response_format,omitempty"`
	MaxTokens      int            `json:"max_tokens,omitempty"`
	Temperature    float64        `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *Client) chatJSON(ctx context.Context, systemPrompt string, userContent any) (string, error) {
	baseURL, apiKey, model, err := c.requireAuth()
	if err != nil {
		return "", err
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userContent},
		},
		ResponseFormat: map[string]any{"type": "json_object"},
		MaxTokens:      2048,
		Temperature:    0.7,
	}

	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/chat/completions", bytes.NewReader(rawReq))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 DeepSeek 接口失败: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode >= 400 {
		return "", formatAPIError(resp.StatusCode, body)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("解析 DeepSeek 响应失败: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("DeepSeek 返回空 choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		// DeepSeek JSON mode caveat: invalid JSON output may be replaced with whitespace.
		return "", fmt.Errorf("DeepSeek 返回了空白 JSON 内容（finish_reason=%s），请重试", parsed.Choices[0].FinishReason)
	}
	return content, nil
}

func (c *Client) requireAuth() (baseURL, apiKey, model string, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if strings.TrimSpace(c.apiKey) == "" {
		return "", "", "", ErrMissingAPIKey
	}
	return c.baseURL, c.apiKey, c.model, nil
}

func buildMultimodalContent(text, imagePath string) (any, error) {
	imagePath = strings.TrimSpace(imagePath)
	if imagePath == "" {
		return text, nil
	}
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return nil, fmt.Errorf("读取图像附件失败: %w", err)
	}
	mimeType := detectImageMIME(imagePath, data)
	dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
	return []map[string]any{
		{"type": "text", "text": text},
		{
			"type": "image_url",
			"image_url": map[string]any{
				"url":    dataURL,
				"detail": "high",
			},
		},
	}, nil
}

func detectImageMIME(path string, data []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		ct := http.DetectContentType(data)
		if strings.HasPrefix(ct, "image/") {
			return ct
		}
		return "image/png"
	}
}

func buildNodeContextPrompt(node domain.IdeaNode, evidences []domain.RawEvidence) string {
	var evLines []string
	for i, ev := range evidences {
		evLines = append(evLines, fmt.Sprintf("%d. [%s | %s | %s] %s",
			i+1, ev.ModalityBadge(), ev.CapturedAt.Format("2006-01-02 15:04"), ev.Scene.SummaryLine(), ev.RawText))
	}
	return fmt.Sprintf(
		"【灵感节点】\n标题：%s\n当前阶段：%s\n口语化摘要：%s\n标签：%s\n\n【原始证据清单（不可篡改的当时记录）】\n%s",
		node.Title, node.Stage.Label(), node.ConversationalSummary,
		strings.Join(node.Tags, "、"), strings.Join(evLines, "\n"),
	)
}

func decodeJSONFlexible(raw string, dst any) error {
	clean := strings.TrimSpace(raw)
	clean = strings.TrimPrefix(clean, "```json")
	clean = strings.TrimPrefix(clean, "```")
	clean = strings.TrimSuffix(clean, "```")
	clean = strings.TrimSpace(clean)
	return json.Unmarshal([]byte(clean), dst)
}

func formatAPIError(status int, body []byte) error {
	var errPayload struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &errPayload); err == nil && errPayload.Error.Message != "" {
		return fmt.Errorf("DeepSeek API 错误 (%d): %s", status, errPayload.Error.Message)
	}
	return fmt.Errorf("DeepSeek API 返回 HTTP %d: %s", status, strings.TrimSpace(string(body)))
}

func normalizeBaseURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		u = config.DefaultDeepSeekBaseURL
	}
	u = strings.TrimRight(u, "/")
	u = strings.TrimSuffix(u, "/v1")
	return u
}

func defaultString(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}

func summarizeFallbackTitle(text string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		return "新的灵感记忆球"
	}
	if len(runes) > 18 {
		return string(runes[:18]) + "…"
	}
	return string(runes)
}
