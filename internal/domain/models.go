// Package domain defines core Neromancer entities and invariants.
// No package inside internal/domain may import store, ai, media, service, or ui.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// OrbTheme defines the gentle, lively Inside-Out inspired colorways for memory orbs.
type OrbTheme string

const (
	ThemeCoral    OrbTheme = "coral"    // 郁金香珊瑚
	ThemeSky      OrbTheme = "sky"      // 晴空蓝
	ThemeAmber    OrbTheme = "amber"    // 琥珀金
	ThemeMint     OrbTheme = "mint"     // 薄荷青
	ThemeLavender OrbTheme = "lavender" // 薰衣草紫
)

// AllThemes lists all supported Memory Orb color themes.
var AllThemes = []OrbTheme{
	ThemeCoral,
	ThemeSky,
	ThemeAmber,
	ThemeMint,
	ThemeLavender,
}

// NormalizeTheme falls back to ThemeCoral for unrecognized theme values.
func NormalizeTheme(raw string) OrbTheme {
	switch OrbTheme(strings.ToLower(strings.TrimSpace(raw))) {
	case ThemeCoral:
		return ThemeCoral
	case ThemeSky:
		return ThemeSky
	case ThemeAmber:
		return ThemeAmber
	case ThemeMint:
		return ThemeMint
	case ThemeLavender:
		return ThemeLavender
	default:
		return ThemeCoral
	}
}

// Label returns a friendly theme descriptor.
func (t OrbTheme) Label() string {
	switch t {
	case ThemeCoral:
		return "珊瑚粉"
	case ThemeSky:
		return "晴空蓝"
	case ThemeAmber:
		return "琥珀金"
	case ThemeMint:
		return "薄荷青"
	case ThemeLavender:
		return "薰衣草"
	default:
		return "珊瑚粉"
	}
}

// Canonical semantic SVG icon keys auto-matched when an idea lands in the vault.
const (
	IconKeyBotanical = "botanical"
	IconKeyCompass   = "compass"
	IconKeyWave      = "wave"
	IconKeyPrism     = "prism"
	IconKeyCode      = "code"
	IconKeyBook      = "book"
	IconKeyMusic     = "music"
	IconKeyCamera    = "camera"
	IconKeyCoffee    = "coffee"
	IconKeyBulb      = "bulb"
)

// VisualMotif captures DeepSeek-generated creative visual cues rendered via vector SVG badges and posters.
type VisualMotif struct {
	MotifType string  `json:"motif_type"` // botanical | compass | wave | prism | code | book | music | camera | coffee | bulb
	AccentHex string  `json:"accent_hex"` // Optional accent override e.g. "#2B6CB0"
	Caption   string  `json:"caption"`    // Poetic micro-caption displayed under the card/detail
	Energy    float64 `json:"energy"`     // 0.0 ~ 1.0 intensity
}

// NormalizeIconKey maps canonical and legacy motif names into canonical icon keys.
// Returns "" if the value is empty or unrecognized.
func NormalizeIconKey(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case IconKeyBotanical, "petals", "sprout", "flower", "plant":
		return IconKeyBotanical
	case IconKeyCompass, "orbit":
		return IconKeyCompass
	case IconKeyWave, "audio", "voice", "sound":
		return IconKeyWave
	case IconKeyPrism, "art", "shader", "visual":
		return IconKeyPrism
	case IconKeyCode, "dev", "tech":
		return IconKeyCode
	case IconKeyBook, "note", "reading":
		return IconKeyBook
	case IconKeyMusic, "song", "melody":
		return IconKeyMusic
	case IconKeyCamera, "photo", "image":
		return IconKeyCamera
	case IconKeyCoffee, "life", "daily":
		return IconKeyCoffee
	case IconKeyBulb, "constellation", "idea", "spark":
		return IconKeyBulb
	default:
		return ""
	}
}

// InferIdeaIconKey automatically picks the most fitting semantic SVG icon key from idea text, tags, and modality.
func InferIdeaIconKey(title, summary string, tags []string, modality string) string {
	haystack := strings.ToLower(strings.TrimSpace(title + " " + summary + " " + strings.Join(tags, " ")))

	rules := []struct {
		key      string
		keywords []string
	}{
		{IconKeyBotanical, []string{"郁金香", "花瓶", "花店", "花盆", "花瓣", "花草", "阳台", "植物", "绿植", "盆栽", "园艺", "薄荷", "多肉", "flower", "tulip", "plant", "botanical"}},
		{IconKeyPrism, []string{"着色", "渲染", "图形", "光影", "手绘", "噪点", "焦散", "配色", "颜色", "视觉", "界面", "设计", "绘画", "插画", "水彩", "shader", "prism", "render", "design", "ui", "color"}},
		{IconKeyWave, []string{"手表", "穿戴交互", "盲操", "语音", "声波", "录音", "麦克风", "白噪音", "耳机", "硬件", "声音", "watch", "voice", "audio", "mic", "wearable"}},
		{IconKeyCompass, []string{"强化学习", "算法", "转移概率", "马尔可夫", "数学", "模型", "推理", "公式", "研究", "动态规划", "蒙特卡洛", "rl", "ml", "model", "algorithm", "math"}},
		{IconKeyMusic, []string{"歌单", "音乐", "步频", "旋律", "歌曲", "节奏", "bpm", "music", "song", "playlist"}},
		{IconKeyCode, []string{"代码", "程序", "架构", "接口", "引擎", "bug", "api", "golang", "rust", "code", "software", "app"}},
		{IconKeyCamera, []string{"拍照", "街拍", "摄影", "延时", "镜头", "相册", "胶片", "画面", "photo", "camera", "film", "video"}},
		{IconKeyBook, []string{"读书", "阅读", "摘录", "文章", "写作", "日记", "单词", "错题", "背词", "笔记", "书单", "书籍", "书页", "book", "read", "note", "essay"}},
		{IconKeyCoffee, []string{"咖啡", "散步", "日常", "生活", "习惯", "厨房", "做饭", "早起", "书桌", "桌宠", "coffee", "walk", "daily", "life", "desk"}},
	}
	for _, r := range rules {
		for _, kw := range r.keywords {
			if strings.Contains(haystack, kw) {
				return r.key
			}
		}
	}

	switch strings.ToLower(strings.TrimSpace(modality)) {
	case "image":
		return IconKeyCamera
	case "voice":
		return IconKeyWave
	case "sketch":
		return IconKeyPrism
	default:
		return IconKeyBulb
	}
}

// InferThemeForIcon selects a harmonious mineral theme colorway for an icon key.
func InferThemeForIcon(iconKey string) OrbTheme {
	switch NormalizeIconKey(iconKey) {
	case IconKeyBotanical:
		return ThemeCoral
	case IconKeyCompass, IconKeyCode:
		return ThemeSky
	case IconKeyWave, IconKeyBook:
		return ThemeMint
	case IconKeyPrism, IconKeyMusic:
		return ThemeLavender
	default:
		return ThemeAmber
	}
}

// ResolveIdeaIconKey returns the canonical icon key for a stored IdeaNode,
// falling back to text-based inference when empty or generic.
func ResolveIdeaIconKey(node IdeaNode) string {
	norm := NormalizeIconKey(node.Visual.MotifType)
	if norm != "" && norm != IconKeyBulb {
		return norm
	}
	inferred := InferIdeaIconKey(node.Title, node.ConversationalSummary, node.Tags, "")
	if inferred != "" {
		return inferred
	}
	if norm != "" {
		return norm
	}
	return IconKeyBulb
}

// InferIdeaFolder maps an idea's text, tags, and semantic icon into a concise human-readable
// folder category so newly captured thoughts naturally settle into a clean file directory.
func InferIdeaFolder(title, summary string, tags []string, iconKey string) string {
	switch NormalizeIconKey(iconKey) {
	case IconKeyBotanical, IconKeyPrism, IconKeyCamera:
		return "器物与光影美学"
	case IconKeyCompass, IconKeyCode:
		return "算法与系统直觉"
	case IconKeyWave, IconKeyMusic:
		return "穿戴与声音交互"
	case IconKeyBook:
		return "阅读与随笔摘录"
	case IconKeyCoffee:
		return "日常习惯与生活"
	}
	inferred := InferIdeaIconKey(title, summary, tags, "")
	switch inferred {
	case IconKeyBotanical, IconKeyPrism, IconKeyCamera:
		return "器物与光影美学"
	case IconKeyCompass, IconKeyCode:
		return "算法与系统直觉"
	case IconKeyWave, IconKeyMusic:
		return "穿戴与声音交互"
	case IconKeyBook:
		return "阅读与随笔摘录"
	case IconKeyCoffee:
		return "日常习惯与生活"
	default:
		return "最近散点闪念"
	}
}

// ResolveIdeaFolder returns the node's AI-assigned folder or falls back to semantic inference.
func ResolveIdeaFolder(node IdeaNode) string {
	if f := strings.TrimSpace(strings.Trim(node.Folder, "/\\#＃")); f != "" {
		return f
	}
	return InferIdeaFolder(node.Title, node.ConversationalSummary, node.Tags, node.Visual.MotifType)
}

// SceneContext stores ambient recall clues captured with zero user typing burden.
type SceneContext struct {
	TimeOfDay   string `json:"time_of_day"`
	Location    string `json:"location"`
	MoodWeather string `json:"mood_weather"`
	NowPlaying  string `json:"now_playing"`
}

// SummaryLine formats non-empty scene cues into a compact readable string.
func (sc SceneContext) SummaryLine() string {
	return joinSceneParts(sc.TimeOfDay, sc.Location, sc.MoodWeather, sc.NowPlaying)
}

// LocationSummaryLine formats scene cues while stripping any leading HH:MM clock prefix
// so timestamps rendered separately never conflict with the scene summary.
func (sc SceneContext) LocationSummaryLine() string {
	tod := stripLeadingClock(sc.TimeOfDay)
	return joinSceneParts(tod, sc.Location, sc.MoodWeather, sc.NowPlaying)
}

func joinSceneParts(items ...string) string {
	var parts []string
	for _, s := range items {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "随手记下"
	}
	return strings.Join(parts, " · ")
}

func stripLeadingClock(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 5 && s[2] == ':' && s[0] >= '0' && s[0] <= '9' && s[1] >= '0' && s[1] <= '9' && s[3] >= '0' && s[3] <= '9' && s[4] >= '0' && s[4] <= '9' {
		return strings.TrimSpace(s[5:])
	}
	return s
}

// IdeaNode represents an organized, deduplicated memory orb inside Mind Space.
type IdeaNode struct {
	ID                    string      `json:"id"`
	ParentID              string      `json:"parent_id,omitempty"`
	Folder                string      `json:"folder,omitempty"`
	Title                 string      `json:"title"`
	ConversationalSummary string      `json:"conversational_summary"`
	Tags                  []string    `json:"tags"`
	Theme                 OrbTheme    `json:"theme"`
	Visual                VisualMotif `json:"visual"`
	Weight                float64     `json:"weight"` // 0.1 ~ 1.0; drives orb radius & glow
	GuidingQuestions      []string    `json:"guiding_questions"`
	ActionPlanMD          string      `json:"action_plan_md"`
	RelatedIDs            []string    `json:"related_ids"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

// RawEvidence is an append-only, immutable record of what the user actually said, wrote, or photographed.
type RawEvidence struct {
	ID         string       `json:"id"`
	NodeID     string       `json:"node_id"`
	Modality   string       `json:"modality"` // "voice" | "text" | "image" | "sketch"
	RawText    string       `json:"raw_text"`
	AudioPath  string       `json:"audio_path,omitempty"`
	ImagePath  string       `json:"image_path,omitempty"`
	DurationMs int          `json:"duration_ms,omitempty"`
	Scene      SceneContext `json:"scene"`
	CapturedAt time.Time    `json:"captured_at"`
}

// ModalityBadge returns a concise icon+label for UI display.
func (ev RawEvidence) ModalityBadge() string {
	switch ev.Modality {
	case "voice":
		return "🎙️ 语音"
	case "image":
		return "🖼️ 图像"
	case "sketch":
		return "✏️ 涂鸦"
	default:
		return "📝 速记"
	}
}

// SleepInsight holds an overnight or background cross-idea connection found by the Sleep Agent.
type SleepInsight struct {
	ID               string    `json:"id"`
	Title            string    `json:"title"`
	ConnectedNodeIDs []string  `json:"connected_node_ids"`
	SparkSummary     string    `json:"spark_summary"`
	SuggestedAction  string    `json:"suggested_action"`
	CreatedAt        time.Time `json:"created_at"`
	Dismissed        bool      `json:"dismissed"`
}

// RoamSlice represents a read-only inspiration snippet from another creator's "idea junkyard".
type RoamSlice struct {
	ID          string   `json:"id"`
	AuthorAlias string   `json:"author_alias"`
	Title       string   `json:"title"`
	Snippet     string   `json:"snippet"`
	DomainTags  []string `json:"domain_tags"`
	Theme       OrbTheme `json:"theme"`
	SceneHint   string   `json:"scene_hint"`
}

// OrbCapsule is a portable .orb.json sharing bundle for cool peer-to-peer sharing without a forum.
type OrbCapsule struct {
	Version    string        `json:"version"`
	ExportedAt time.Time     `json:"exported_at"`
	Node       IdeaNode      `json:"node"`
	Evidences  []RawEvidence `json:"evidences"`
}

// RecallMatch represents a fast local associative recall hit while typing or reviewing.
type RecallMatch struct {
	Node          IdeaNode `json:"node"`
	MatchedText   string   `json:"matched_text"`
	MatchReason   string   `json:"match_reason"`
	EvidenceCount int      `json:"evidence_count"`
}

// Validate checks minimal invariants on an IdeaNode before storage.
func (n IdeaNode) Validate() error {
	if strings.TrimSpace(n.ID) == "" {
		return fmt.Errorf("idea node id cannot be empty")
	}
	if strings.TrimSpace(n.Title) == "" {
		return fmt.Errorf("idea node title cannot be empty")
	}
	return nil
}
