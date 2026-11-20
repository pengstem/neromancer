package service

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/store"
)

// TagCount represents an active tag pill with usage count.
type TagCount struct {
	Tag   string
	Count int
}

// OrganizeService manages Mind Space tree structure, stage evolution, and associative recall.
type OrganizeService struct {
	store *store.Store
}

// NewOrganizeService creates an OrganizeService backed by store.
func NewOrganizeService(st *store.Store) *OrganizeService {
	return &OrganizeService{store: st}
}

// ListVault returns all nodes, evidence counts per node, and root/child tree mappings.
func (s *OrganizeService) ListVault() (nodes []domain.IdeaNode, counts map[string]int, roots []string, children map[string][]string, err error) {
	nodes, err = s.store.ListNodes()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	counts, err = s.store.CountEvidencesByNode()
	if err != nil {
		return nil, nil, nil, nil, err
	}

	byID := make(map[string]domain.IdeaNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	children = make(map[string][]string)
	for _, n := range nodes {
		if n.ParentID == "" || n.ParentID == n.ID {
			roots = append(roots, n.ID)
			continue
		}
		if _, ok := byID[n.ParentID]; !ok {
			roots = append(roots, n.ID)
			continue
		}
		children[n.ParentID] = append(children[n.ParentID], n.ID)
	}
	return nodes, counts, roots, children, nil
}

// SearchRecall performs instant local associative recall as the user types or reviews.
func (s *OrganizeService) SearchRecall(query string, limit int) ([]domain.RecallMatch, error) {
	return s.store.SearchAssociativeRecall(query, limit)
}

// AdvanceStage moves a node to stage and records an immutable StageSnapshot.
func (s *OrganizeService) AdvanceStage(nodeID string, stage domain.LifecycleStage, note string) (domain.IdeaNode, domain.StageSnapshot, error) {
	return s.store.UpdateNodeStage(nodeID, stage, note)
}

// MergeNodes merges sourceID into targetID while keeping every RawEvidence intact.
func (s *OrganizeService) MergeNodes(sourceID, targetID string) (domain.IdeaNode, error) {
	if sourceID == targetID {
		return domain.IdeaNode{}, fmt.Errorf("不能将节点合并到自身")
	}
	src, err := s.store.GetNode(sourceID)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	dst, err := s.store.GetNode(targetID)
	if err != nil {
		return domain.IdeaNode{}, err
	}

	evs, err := s.store.ListEvidencesByNode(sourceID)
	if err != nil {
		return domain.IdeaNode{}, err
	}
	for _, ev := range evs {
		if err := s.store.ReassignEvidenceNode(ev.ID, targetID); err != nil {
			return domain.IdeaNode{}, err
		}
	}

	dst.Tags = mergeUniqueStrings(dst.Tags, src.Tags)
	if dst.Stage == domain.StageSpark {
		dst.Stage = domain.StageSprouting
	}
	dst.Weight = min(1.0, dst.Weight+0.08)
	dst.UpdatedAt = time.Now()
	if err := s.store.UpsertNode(dst); err != nil {
		return domain.IdeaNode{}, err
	}
	_ = s.store.AddStageSnapshot(domain.StageSnapshot{
		ID:        fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		NodeID:    dst.ID,
		Stage:     dst.Stage,
		Title:     dst.Title,
		Summary:   dst.ConversationalSummary,
		Note:      fmt.Sprintf("手动归并《%s》的原始记录", src.Title),
		CreatedAt: time.Now(),
	})
	_ = s.store.DeleteNode(sourceID)
	return dst, nil
}

// CollectTags aggregates unique tags sorted by frequency descending.
func (s *OrganizeService) CollectTags(nodes []domain.IdeaNode) []TagCount {
	counts := make(map[string]int)
	for _, n := range nodes {
		for _, tag := range n.Tags {
			t := strings.TrimSpace(tag)
			if t != "" {
				counts[t]++
			}
		}
	}
	out := make([]TagCount, 0, len(counts))
	for tag, c := range counts {
		out = append(out, TagCount{Tag: tag, Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}

// KeywordCluster summarizes a weighted thematic group for the Bento Dashboard matrix.
type KeywordCluster struct {
	Name        string
	FilterTag   string
	SubKeywords string
	IconKey     string
	Theme       domain.OrbTheme
	Count       int
	Percent     int
	Share       float32
}

// DashboardClue represents an actionable sleep connection or guiding question surfaced on the Dashboard.
type DashboardClue struct {
	Kind    string // "sleep" | "question"
	NodeID  string
	IconKey string
	Theme   domain.OrbTheme
	Text    string
}

// DashboardDigest powers the Homepage Bento Dashboard with live vault analytics and clues.
type DashboardDigest struct {
	PersonaHeadline string
	PersonaSummary  string
	TotalNodes      int
	TotalEvidences  int
	Sparkline       []float32
	Clusters        []KeywordCluster
	Clues           []DashboardClue
}

// BuildDashboardDigest computes weighted keyword clusters, activity sparkline, and next clues from vault data.
func (s *OrganizeService) BuildDashboardDigest(nodes []domain.IdeaNode, evCounts map[string]int, insights []domain.SleepInsight) DashboardDigest {
	digest := DashboardDigest{
		TotalNodes: len(nodes),
		Sparkline:  computeActivitySparkline(nodes, time.Now()),
	}
	for _, n := range nodes {
		c := evCounts[n.ID]
		if c <= 0 {
			c = 1
		}
		digest.TotalEvidences += c
	}

	if len(nodes) == 0 {
		digest.PersonaHeadline = "开始记录你的第一个想法"
		digest.PersonaSummary = "随口说一句话或拍张图，系统会自动归类并提炼关键词。"
		return digest
	}

	type clusterAcc struct {
		name      string
		filterTag string
		subTags   []string
		subSeen   map[string]struct{}
		iconKey   string
		theme     domain.OrbTheme
		bestNodeW float64
		score     float64
		count     int
	}

	byID := make(map[string]domain.IdeaNode, len(nodes))
	groups := make(map[string]*clusterAcc)
	var groupKeys []string

	activeCount := 0
	for _, n := range nodes {
		byID[n.ID] = n
		if n.Stage != domain.StageSpark {
			activeCount++
		}

		primaryTag := ""
		var secondary []string
		for _, rawTag := range n.Tags {
			t := strings.TrimSpace(rawTag)
			if t == "" {
				continue
			}
			if primaryTag == "" {
				primaryTag = t
			} else {
				secondary = append(secondary, t)
			}
		}

		groupKey := primaryTag
		displayName := primaryTag
		if groupKey == "" {
			groupKey = "_untagged"
			displayName = "随手闪念"
		}

		evC := max(1, evCounts[n.ID])
		nodeScore := max(0.35, n.Weight) + 0.16*float64(evC)

		acc, exists := groups[groupKey]
		if !exists {
			acc = &clusterAcc{
				name:      displayName,
				filterTag: primaryTag,
				subSeen:   make(map[string]struct{}),
				iconKey:   domain.ResolveIdeaIconKey(n),
				theme:     n.Theme,
				bestNodeW: nodeScore,
			}
			groups[groupKey] = acc
			groupKeys = append(groupKeys, groupKey)
		} else if nodeScore > acc.bestNodeW {
			acc.bestNodeW = nodeScore
			acc.iconKey = domain.ResolveIdeaIconKey(n)
			acc.theme = n.Theme
		}
		acc.score += nodeScore
		acc.count++
		for _, st := range secondary {
			if _, ok := acc.subSeen[st]; !ok && len(acc.subTags) < 3 {
				acc.subSeen[st] = struct{}{}
				acc.subTags = append(acc.subTags, st)
			}
		}
	}

	ranked := make([]*clusterAcc, 0, len(groupKeys))
	for _, k := range groupKeys {
		ranked = append(ranked, groups[k])
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].name < ranked[j].name
	})

	limit := min(len(ranked), 4)
	ranked = ranked[:limit]

	totalScore := 0.0
	maxScore := 0.0
	for _, acc := range ranked {
		totalScore += acc.score
		if acc.score > maxScore {
			maxScore = acc.score
		}
	}

	pctSum := 0
	for i, acc := range ranked {
		pct := 0
		if totalScore > 0 {
			if i == len(ranked)-1 {
				pct = max(0, 100-pctSum)
			} else {
				pct = int(math.Round((acc.score / totalScore) * 100))
				if pct == 0 {
					pct = 1
				}
				pctSum += pct
			}
		}
		share := float32(0.32)
		if maxScore > 0 {
			share = float32(0.28 + 0.64*(acc.score/maxScore))
		}
		sub := strings.Join(acc.subTags, " · ")
		if sub == "" {
			sub = fmt.Sprintf("%d 篇灵感记录", acc.count)
		}
		digest.Clusters = append(digest.Clusters, KeywordCluster{
			Name:        acc.name,
			FilterTag:   acc.filterTag,
			SubKeywords: sub,
			IconKey:     acc.iconKey,
			Theme:       acc.theme,
			Count:       acc.count,
			Percent:     pct,
			Share:       share,
		})
	}

	if len(digest.Clusters) >= 2 {
		c0, c1 := digest.Clusters[0], digest.Clusters[1]
		digest.PersonaHeadline = fmt.Sprintf("%s × %s", c0.Name, c1.Name)
		digest.PersonaSummary = fmt.Sprintf(
			"共沉淀 %d 篇灵感（%d 篇已推进），重心聚焦在「%s」与「%s」（%d%%）。",
			len(nodes), activeCount, c0.Name, c1.Name, c0.Percent+c1.Percent,
		)
	} else {
		c0 := digest.Clusters[0]
		digest.PersonaHeadline = fmt.Sprintf("专注 %s", c0.Name)
		digest.PersonaSummary = fmt.Sprintf(
			"已记录 %d 篇灵感与 %d 条原始记录，当前都围绕「%s」展开。",
			len(nodes), digest.TotalEvidences, c0.Name,
		)
	}

	// Build up to 2 actionable clues: active sleep insight + top guiding question.
	sleepNodeID := ""
	if len(insights) > 0 {
		ins := insights[0]
		iconKey := domain.IconKeyBulb
		theme := domain.ThemeAmber
		for _, id := range ins.ConnectedNodeIDs {
			if n, ok := byID[id]; ok {
				sleepNodeID = n.ID
				iconKey = domain.ResolveIdeaIconKey(n)
				theme = n.Theme
				break
			}
		}
		digest.Clues = append(digest.Clues, DashboardClue{
			Kind:    "sleep",
			NodeID:  sleepNodeID,
			IconKey: iconKey,
			Theme:   theme,
			Text:    ins.Title,
		})
	}

	var qNode *domain.IdeaNode
	for i := range nodes {
		n := &nodes[i]
		if len(n.GuidingQuestions) == 0 {
			continue
		}
		if qNode == nil || n.ID != sleepNodeID {
			qNode = n
			if n.ID != sleepNodeID {
				break
			}
		}
	}
	if qNode != nil {
		digest.Clues = append(digest.Clues, DashboardClue{
			Kind:    "question",
			NodeID:  qNode.ID,
			IconKey: domain.ResolveIdeaIconKey(*qNode),
			Theme:   qNode.Theme,
			Text:    "追问：" + qNode.GuidingQuestions[0],
		})
	}

	return digest
}

func computeActivitySparkline(nodes []domain.IdeaNode, now time.Time) []float32 {
	const buckets = 6
	counts := make([]float64, buckets)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	for _, n := range nodes {
		ts := n.CreatedAt
		if ts.IsZero() {
			ts = n.UpdatedAt
		}
		if ts.IsZero() {
			counts[buckets-1]++
			continue
		}
		day := time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, now.Location())
		daysAgo := int(today.Sub(day).Hours() / 24)
		idx := buckets - 1 - daysAgo
		if idx < 0 {
			idx = 0
		}
		if idx >= buckets {
			idx = buckets - 1
		}
		counts[idx] += 1.0
	}

	// Cumulative trend gently weighted with per-day activity for a clean pulse sparkline.
	series := make([]float64, buckets)
	running := 0.0
	maxVal := 1.0
	for i := 0; i < buckets; i++ {
		running += counts[i]*0.85 + 0.25
		series[i] = running
		if running > maxVal {
			maxVal = running
		}
	}

	pts := make([]float32, buckets)
	for i, v := range series {
		norm := float32(0.18 + 0.72*(v/maxVal))
		if norm > 0.94 {
			norm = 0.94
		}
		pts[i] = norm
	}
	return pts
}
