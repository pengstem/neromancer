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
		tags := inferQuickTags(text, modality)
		iconKey := domain.InferIdeaIconKey(text, summary, tags, modality)
		existingNodes, _ := s.store.ListNodes()
		folder, relatedIDs := inferLocalFolderAndLinks(title, summary, tags, iconKey, existingNodes)
		node = domain.IdeaNode{
			ID:                    fmt.Sprintf("orb_%d", now.UnixNano()),
			Folder:                folder,
			Title:                 title,
			ConversationalSummary: summary,
			Tags:                  tags,
			Theme:                 domain.InferThemeForIcon(iconKey),
			Visual: domain.VisualMotif{
				MotifType: iconKey,
				Caption:   "",
				Energy:    0.72,
			},
			Weight:     0.62,
			RelatedIDs: relatedIDs,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := s.store.UpsertNode(node); err != nil {
			return nil, err
		}
		s.syncBidirectionalLinks(node.ID, relatedIDs)
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
			Folder:  domain.ResolveIdeaFolder(n),
			Title:   n.Title,
			Summary: n.ConversationalSummary,
			Tags:    n.Tags,
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
			if strings.TrimSpace(decision.Folder) != "" {
				target.Folder = strings.TrimSpace(decision.Folder)
			}
			if strings.TrimSpace(decision.Title) != "" {
				target.Title = strings.TrimSpace(decision.Title)
			}
			if strings.TrimSpace(decision.ConversationalSummary) != "" {
				target.ConversationalSummary = strings.TrimSpace(decision.ConversationalSummary)
			}
			target.Tags = mergeUniqueTags(target.Tags, decision.Tags, 4)
			target.Weight = math.Min(1.0, math.Max(target.Weight, decision.Weight)+0.08)
			if len(decision.GuidingQuestions) > 0 {
				target.GuidingQuestions = decision.GuidingQuestions
			}
			target.Visual = resolveOrganizedVisual(target.Visual, decision.Visual, target.Title, target.ConversationalSummary, target.Tags, ev.Modality)
			target.RelatedIDs = filterSelfID(target.ID, mergeUniqueStrings(target.RelatedIDs, decision.RelatedIDs))
			target.UpdatedAt = now
			if err := s.store.UpsertNode(target); err != nil {
				return domain.IdeaNode{}, "", err
			}
			s.syncBidirectionalLinks(target.ID, target.RelatedIDs)
			return target, target.ID, nil
		}
	}

	node, err := s.store.GetNode(currentNodeID)
	if err != nil {
		return domain.IdeaNode{}, "", err
	}
	if strings.TrimSpace(decision.Folder) != "" {
		node.Folder = strings.TrimSpace(decision.Folder)
	}
	if strings.TrimSpace(decision.Title) != "" {
		node.Title = strings.TrimSpace(decision.Title)
	}
	if strings.TrimSpace(decision.ConversationalSummary) != "" {
		node.ConversationalSummary = strings.TrimSpace(decision.ConversationalSummary)
	}
	if len(decision.Tags) > 0 {
		node.Tags = mergeUniqueTags(nil, decision.Tags, 4)
	}
	if decision.Theme != "" {
		node.Theme = domain.NormalizeTheme(decision.Theme)
	}
	if decision.Weight > 0 {
		node.Weight = math.Min(1.0, decision.Weight)
	}
	node.Visual = resolveOrganizedVisual(node.Visual, decision.Visual, node.Title, node.ConversationalSummary, node.Tags, ev.Modality)
	if strings.TrimSpace(node.Folder) == "" {
		node.Folder = domain.InferIdeaFolder(node.Title, node.ConversationalSummary, node.Tags, node.Visual.MotifType)
	}
	if len(decision.GuidingQuestions) > 0 {
		node.GuidingQuestions = decision.GuidingQuestions
	}
	if len(decision.RelatedIDs) > 0 {
		node.RelatedIDs = filterSelfID(node.ID, mergeUniqueStrings(node.RelatedIDs, decision.RelatedIDs))
	}
	node.UpdatedAt = now
	if err := s.store.UpsertNode(node); err != nil {
		return domain.IdeaNode{}, "", err
	}
	s.syncBidirectionalLinks(node.ID, node.RelatedIDs)
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

func mergeUniqueTags(primary, secondary []string, maxCount int) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	var out []string
	for _, list := range [][]string{secondary, primary} {
		for _, item := range list {
			t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(item), "#＃"))
			if t == "" {
				continue
			}
			if _, ok := seen[t]; !ok {
				seen[t] = struct{}{}
				out = append(out, t)
				if maxCount > 0 && len(out) >= maxCount {
					return out
				}
			}
		}
	}
	return out
}

func inferQuickTags(text, modality string) []string {
	var matched []string
	hay := strings.ToLower(strings.TrimSpace(text))
	tagHints := []struct {
		tag string
		kws []string
	}{
		{"阳台植物", []string{"花", "郁金香", "薄荷", "绿植", "阳台", "花盆", "花瓶", "浇水"}},
		{"光影质感", []string{"玻璃", "光影", "着色", "噪点", "手绘", "焦散", "颜色", "水彩", "shader"}},
		{"强化学习", []string{"强化学习", "转移概率", "马尔可夫", "蒙特卡洛", "动态规划", "采样", "rl"}},
		{"穿戴交互", []string{"手表", "盲操", "录音", "语音", "抬腕", "硬件"}},
		{"图形学", []string{"渲染", "管线", "多边形", "法线", "引擎"}},
		{"生活美学", []string{"咖啡", "散步", "书桌", "窗帘", "清晨", "日常"}},
	}
	for _, h := range tagHints {
		for _, kw := range h.kws {
			if strings.Contains(hay, kw) {
				matched = append(matched, h.tag)
				break
			}
		}
		if len(matched) >= 2 {
			break
		}
	}
	if len(matched) == 0 {
		return []string{modalityTag(modality)}
	}
	return matched
}

func inferLocalFolderAndLinks(title, summary string, tags []string, iconKey string, existing []domain.IdeaNode) (string, []string) {
	probe := domain.IdeaNode{
		Title:                 title,
		ConversationalSummary: summary,
		Tags:                  tags,
		Visual:                domain.VisualMotif{MotifType: iconKey},
	}
	var related []string
	folderVotes := make(map[string]int)
	for _, other := range existing {
		if notesNaturallyRelated(probe, other) {
			related = append(related, other.ID)
			f := domain.ResolveIdeaFolder(other)
			if f != "" && f != "最近散点闪念" {
				folderVotes[f]++
			}
		}
	}
	bestFolder := ""
	bestCount := 0
	for f, c := range folderVotes {
		if c > bestCount {
			bestCount = c
			bestFolder = f
		}
	}
	if bestFolder == "" {
		bestFolder = domain.InferIdeaFolder(title, summary, tags, iconKey)
	}
	if len(related) > 4 {
		related = related[:4]
	}
	return bestFolder, related
}

func (s *CaptureService) syncBidirectionalLinks(nodeID string, relatedIDs []string) {
	if s.store == nil || nodeID == "" {
		return
	}
	for _, relID := range relatedIDs {
		relID = strings.TrimSpace(relID)
		if relID == "" || relID == nodeID {
			continue
		}
		other, err := s.store.GetNode(relID)
		if err != nil {
			continue
		}
		updated := filterSelfID(other.ID, mergeUniqueStrings(other.RelatedIDs, []string{nodeID}))
		if len(updated) != len(other.RelatedIDs) {
			other.RelatedIDs = updated
			_ = s.store.UpsertNode(other)
		}
	}
}

func filterSelfID(selfID string, ids []string) []string {
	var out []string
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || id == selfID {
			continue
		}
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}
