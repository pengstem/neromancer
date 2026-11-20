// Package service orchestrates Neromancer capture, deduplicating organization,
// incubation, associative recall, and portable sharing workflows.
package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/store"
)

// QuickCaptureInput describes a low-friction thought captured from the FAB or floating orb.
type QuickCaptureInput struct {
	RawText      string
	Modality     string // "voice" | "text" | "image" | "sketch"
	AudioSrcPath string
	ImageSrcPath string
	TargetNodeID string
	Scene        domain.SceneContext
	AutoOrganize bool
}

// CaptureOutcome summarizes what was saved locally and whether DeepSeek enriched/merged it.
type CaptureOutcome struct {
	Node         domain.IdeaNode
	Evidence     domain.RawEvidence
	MergedIntoID string
	AIWarning    string
}

// CaptureService handles zero-friction raw evidence capture and optional DeepSeek organization.
type CaptureService struct {
	store *store.Store
	ai    *ai.Client
	media *media.Manager
}

// NewCaptureService constructs a CaptureService.
func NewCaptureService(st *store.Store, aiClient *ai.Client, mm *media.Manager) *CaptureService {
	return &CaptureService{store: st, ai: aiClient, media: mm}
}

// CaptureQuick persists the raw capture locally first (<5ms, zero data loss),
// then optionally transcribes audio and runs DeepSeek auto-merge/organization.
func (s *CaptureService) CaptureQuick(ctx context.Context, in QuickCaptureInput) (*CaptureOutcome, error) {
	now := time.Now()
	text := strings.TrimSpace(in.RawText)
	if text == "" && strings.TrimSpace(in.AudioSrcPath) == "" && strings.TrimSpace(in.ImageSrcPath) == "" {
		return nil, fmt.Errorf("先说句话、写几个字，或者选张图吧")
	}

	var managedAudio, managedImage string
	var err error
	if p := strings.TrimSpace(in.AudioSrcPath); p != "" && s.media != nil {
		managedAudio, err = s.media.ImportFile(p, true)
		if err != nil {
			return nil, fmt.Errorf("保存原始音频失败: %w", err)
		}
	}
	if p := strings.TrimSpace(in.ImageSrcPath); p != "" && s.media != nil {
		managedImage, err = s.media.ImportFile(p, false)
		if err != nil {
			return nil, fmt.Errorf("保存原始图片失败: %w", err)
		}
	}

	modality := strings.TrimSpace(in.Modality)
	if modality == "" {
		switch {
		case managedAudio != "":
			modality = "voice"
		case managedImage != "":
			modality = "image"
		default:
			modality = "text"
		}
	}

	var aiWarning string
	if managedAudio != "" && text == "" {
		if s.ai != nil && s.ai.Configured() {
			asr, err := s.ai.TranscribeAudio(ctx, managedAudio)
			if err != nil {
				aiWarning = err.Error()
			} else if asr.Text != "" {
				text = asr.Text
			}
		} else {
			aiWarning = ai.ErrMissingAPIKey.Error()
		}
	}

	if strings.TrimSpace(in.Scene.TimeOfDay) == "" {
		in.Scene.TimeOfDay = timeOfDayLabel(now)
	}

	// Step A: Zero-friction local commit of RawEvidence + target/provisional node.
	var node domain.IdeaNode
	isProvisional := false
	if strings.TrimSpace(in.TargetNodeID) != "" {
		existing, err := s.store.GetNode(in.TargetNodeID)
		if err != nil {
			return nil, err
		}
		node = existing
		node.UpdatedAt = now
		if err := s.store.UpsertNode(node); err != nil {
			return nil, err
		}
	} else {
		isProvisional = true
		title := deriveQuickTitle(text, modality)
		summary := defaultSummary(text, modality)
		tags := []string{modalityTag(modality)}
		iconKey := domain.InferIdeaIconKey(text, summary, tags, modality)
		node = domain.IdeaNode{
			ID:                    fmt.Sprintf("orb_%d", now.UnixNano()),
			Title:                 title,
			ConversationalSummary: summary,
			Tags:                  tags,
			Theme:                 domain.InferThemeForIcon(iconKey),
			Visual: domain.VisualMotif{
				MotifType: iconKey,
				Caption:   "",
				Energy:    0.72,
			},
			Weight:    0.62,
			Stage:     domain.StageSpark,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.store.UpsertNode(node); err != nil {
			return nil, err
		}
	}

	ev := domain.RawEvidence{
		ID:         fmt.Sprintf("ev_%d", now.UnixNano()),
		NodeID:     node.ID,
		Modality:   modality,
		RawText:    text,
		AudioPath:  managedAudio,
		ImagePath:  managedImage,
		Scene:      in.Scene,
		CapturedAt: now,
	}
	if err := s.store.AddRawEvidence(ev); err != nil {
		return nil, err
	}

	outcome := &CaptureOutcome{
		Node:      node,
		Evidence:  ev,
		AIWarning: aiWarning,
	}

	if !in.AutoOrganize || !isProvisional {
		return outcome, nil
	}
	if s.ai == nil || !s.ai.Configured() {
		if outcome.AIWarning == "" {
			outcome.AIWarning = ai.ErrMissingAPIKey.Error()
		}
		return outcome, nil
	}

	// Step B: DeepSeek autonomous deduplication/merge & visual enrichment.
	organizedNode, mergedInto, err := s.OrganizeEvidenceIntoCatalog(ctx, ev, node.ID)
	if err != nil {
		outcome.AIWarning = err.Error()
		return outcome, nil
	}
	ev.NodeID = organizedNode.ID
	outcome.Node = organizedNode
	outcome.Evidence = ev
	outcome.MergedIntoID = mergedInto
	return outcome, nil
}

// OrganizeEvidenceIntoCatalog sends an evidence and current node digests to DeepSeek,
// merging into a matching node (like the two Tulip notes) or enriching the newly created orb.
func (s *CaptureService) OrganizeEvidenceIntoCatalog(ctx context.Context, ev domain.RawEvidence, currentNodeID string) (domain.IdeaNode, string, error) {
	allNodes, err := s.store.ListNodes()
	if err != nil {
		return domain.IdeaNode{}, "", err
	}
	var digests []ai.NodeDigest
	for _, n := range allNodes {
		if n.ID == currentNodeID {
			continue
		}
		digests = append(digests, ai.NodeDigest{
			ID:      n.ID,
			Title:   n.Title,
			Summary: n.ConversationalSummary,
			Tags:    n.Tags,
			Stage:   string(n.Stage),
		})
	}

	decision, err := s.ai.IngestAndOrganize(ctx, ev, digests)
	if err != nil {
		return domain.IdeaNode{}, "", err
	}

	now := time.Now()
	if strings.EqualFold(decision.Action, "merge") && strings.TrimSpace(decision.TargetNodeID) != "" {
		if target, err := s.store.GetNode(decision.TargetNodeID); err == nil {
			if err := s.store.ReassignEvidenceNode(ev.ID, target.ID); err != nil {
				return domain.IdeaNode{}, "", err
			}
			if currentNodeID != "" && currentNodeID != target.ID {
				_ = s.store.DeleteNode(currentNodeID)
			}
			if strings.TrimSpace(decision.Title) != "" {
				target.Title = strings.TrimSpace(decision.Title)
			}
			if strings.TrimSpace(decision.ConversationalSummary) != "" {
				target.ConversationalSummary = strings.TrimSpace(decision.ConversationalSummary)
			}
			target.Tags = mergeUniqueStrings(target.Tags, decision.Tags)
			target.Weight = math.Min(1.0, math.Max(target.Weight, decision.Weight)+0.08)
			if target.Stage == domain.StageSpark {
				target.Stage = domain.StageSprouting
			}
			if len(decision.GuidingQuestions) > 0 {
				target.GuidingQuestions = decision.GuidingQuestions
			}
			target.Visual = resolveOrganizedVisual(target.Visual, decision.Visual, target.Title, target.ConversationalSummary, target.Tags, ev.Modality)
			target.RelatedIDs = mergeUniqueStrings(target.RelatedIDs, decision.RelatedIDs)
			target.UpdatedAt = now
			if err := s.store.UpsertNode(target); err != nil {
				return domain.IdeaNode{}, "", err
			}
			_ = s.store.AddStageSnapshot(domain.StageSnapshot{
				ID:        fmt.Sprintf("snap_%d", now.UnixNano()),
				NodeID:    target.ID,
				Stage:     target.Stage,
				Title:     target.Title,
				Summary:   target.ConversationalSummary,
				Note:      "AI 自动合并了相近的想法",
				CreatedAt: now,
			})
			return target, target.ID, nil
		}
	}

	node, err := s.store.GetNode(currentNodeID)
	if err != nil {
		return domain.IdeaNode{}, "", err
	}
	if strings.TrimSpace(decision.Title) != "" {
		node.Title = strings.TrimSpace(decision.Title)
	}
	if strings.TrimSpace(decision.ConversationalSummary) != "" {
		node.ConversationalSummary = strings.TrimSpace(decision.ConversationalSummary)
	}
	if len(decision.Tags) > 0 {
		node.Tags = decision.Tags
	}
	if decision.Theme != "" {
		node.Theme = domain.NormalizeTheme(decision.Theme)
	}
	if decision.Weight > 0 {
		node.Weight = math.Min(1.0, decision.Weight)
	}
	node.Visual = resolveOrganizedVisual(node.Visual, decision.Visual, node.Title, node.ConversationalSummary, node.Tags, ev.Modality)
	if len(decision.GuidingQuestions) > 0 {
		node.GuidingQuestions = decision.GuidingQuestions
	}
	if len(decision.RelatedIDs) > 0 {
		node.RelatedIDs = decision.RelatedIDs
	}
	node.UpdatedAt = now
	if err := s.store.UpsertNode(node); err != nil {
		return domain.IdeaNode{}, "", err
	}
	_ = s.store.AddStageSnapshot(domain.StageSnapshot{
		ID:        fmt.Sprintf("snap_%d", now.UnixNano()),
		NodeID:    node.ID,
		Stage:     node.Stage,
		Title:     node.Title,
		Summary:   node.ConversationalSummary,
		Note:      "AI 完成了初次整理",
		CreatedAt: now,
	})
	return node, "", nil
}

func resolveOrganizedVisual(current, incoming domain.VisualMotif, title, summary string, tags []string, modality string) domain.VisualMotif {
	out := current
	if incoming.AccentHex != "" {
		out.AccentHex = incoming.AccentHex
	}
	if strings.TrimSpace(incoming.Caption) != "" {
		out.Caption = strings.TrimSpace(incoming.Caption)
	}
	if incoming.Energy > 0 {
		out.Energy = incoming.Energy
	}
	norm := domain.NormalizeIconKey(incoming.MotifType)
	if norm == "" {
		norm = domain.InferIdeaIconKey(title, summary, tags, modality)
	}
	out.MotifType = norm
	return out
}

func timeOfDayLabel(t time.Time) string {
	h := t.Hour()
	switch {
	case h < 6:
		return "深夜"
	case h < 11:
		return "早晨"
	case h < 14:
		return "午间"
	case h < 18:
		return "午后"
	default:
		return "夜晚"
	}
}

func deriveQuickTitle(text, modality string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) == 0 {
		if modality == "voice" {
			return "一段语音记录"
		}
		if modality == "image" {
			return "一张随手拍的图片"
		}
		return "随手一记"
	}
	if len(runes) > 18 {
		return string(runes[:18]) + "…"
	}
	return string(runes)
}

func defaultSummary(text, modality string) string {
	if strings.TrimSpace(text) != "" {
		return text
	}
	if modality == "voice" {
		return "录音已保存好，填好 DeepSeek Key 后就能自动转成文字。"
	}
	return "图片已保存好，随时可以补两句想法或让 AI 帮你看看。"
}

func modalityTag(modality string) string {
	switch modality {
	case "voice":
		return "语音闪念"
	case "image":
		return "图像灵感"
	case "sketch":
		return "手绘草图"
	default:
		return "随口一记"
	}
}

func mergeUniqueStrings(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	var out []string
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, ok := seen[item]; !ok {
				seen[item] = struct{}{}
				out = append(out, item)
			}
		}
	}
	return out
}
