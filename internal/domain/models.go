// Package domain defines core Neromancer entities and invariants.
// No package inside internal/domain may import store, ai, media, service, or ui.
package domain

import (
	"fmt"
	"strings"
	"time"
)

// LifecycleStage represents the maturation phase of an inspiration orb.
type LifecycleStage string

const (
	StageSpark        LifecycleStage = "spark"        // 闪念（初生）
	StageSprouting    LifecycleStage = "sprouting"    // 萌芽（已归并/追问）
	StageIncubating   LifecycleStage = "incubating"   // 孵化中（生成方案/多模态展开）
	StageCrystallized LifecycleStage = "crystallized" // 已沉淀（落地/高权重核心球）
)

// AllStages lists lifecycle stages in natural progression order.
var AllStages = []LifecycleStage{
	StageSpark,
	StageSprouting,
	StageIncubating,
	StageCrystallized,
}

// Label returns the human-readable Chinese label for a lifecycle stage.
func (s LifecycleStage) Label() string {
	switch s {
	case StageSpark:
		return "闪念"
	case StageSprouting:
		return "萌芽"
	case StageIncubating:
		return "孵化中"
	case StageCrystallized:
		return "已沉淀"
	default:
		return "闪念"
	}
}

// Next returns the next stage in the lifecycle, wrapping back to StageSpark at the end.
func (s LifecycleStage) Next() LifecycleStage {
	switch s {
	case StageSpark:
		return StageSprouting
	case StageSprouting:
		return StageIncubating
	case StageIncubating:
		return StageCrystallized
	default:
		return StageSpark
	}
}

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

// VisualMotif captures DeepSeek-generated creative visual cues rendered via GPU Painter and PNG cards.
type VisualMotif struct {
	MotifType string  `json:"motif_type"` // petals | orbit | wave | prism | sprout | constellation
	AccentHex string  `json:"accent_hex"` // Optional accent override e.g. "#2B6CB0"
	Caption   string  `json:"caption"`    // Poetic micro-caption displayed under the orb
	Energy    float64 `json:"energy"`     // 0.0 ~ 1.0 pulse/glow intensity
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
	var parts []string
	for _, s := range []string{sc.TimeOfDay, sc.Location, sc.MoodWeather, sc.NowPlaying} {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	if len(parts) == 0 {
		return "随手记下"
	}
	return strings.Join(parts, " · ")
}

// IdeaNode represents an organized, deduplicated memory orb inside Mind Space.
type IdeaNode struct {
	ID                    string         `json:"id"`
	ParentID              string         `json:"parent_id,omitempty"`
	Title                 string         `json:"title"`
	ConversationalSummary string         `json:"conversational_summary"`
	Tags                  []string       `json:"tags"`
	Theme                 OrbTheme       `json:"theme"`
	Visual                VisualMotif    `json:"visual"`
	Weight                float64        `json:"weight"` // 0.1 ~ 1.0; drives orb radius & glow
	Stage                 LifecycleStage `json:"stage"`
	GuidingQuestions      []string       `json:"guiding_questions"`
	ActionPlanMD          string         `json:"action_plan_md"`
	RelatedIDs            []string       `json:"related_ids"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
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

// StageSnapshot preserves how an inspiration evolved across lifecycle stages.
type StageSnapshot struct {
	ID        string         `json:"id"`
	NodeID    string         `json:"node_id"`
	Stage     LifecycleStage `json:"stage"`
	Title     string         `json:"title"`
	Summary   string         `json:"summary"`
	Note      string         `json:"note"`
	CreatedAt time.Time      `json:"created_at"`
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
	Version    string          `json:"version"`
	ExportedAt time.Time       `json:"exported_at"`
	Node       IdeaNode        `json:"node"`
	Evidences  []RawEvidence   `json:"evidences"`
	Snapshots  []StageSnapshot `json:"snapshots"`
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
