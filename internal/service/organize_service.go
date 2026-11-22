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

// OrganizeService manages Mind Space tree structure and associative recall.
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
	dst.Weight = min(1.0, dst.Weight+0.08)
	dst.UpdatedAt = time.Now()
	if err := s.store.UpsertNode(dst); err != nil {
		return domain.IdeaNode{}, err
	}
	_ = s.store.DeleteNode(sourceID)
	return dst, nil
}

// CollectTags aggregates unique tags sorted by frequency descending.
func (s *OrganizeService) CollectTags(nodes []domain.IdeaNode) []TagCount {
	counts := make(map[string]int)
	for _, n := range nodes {
		seenInNode := make(map[string]struct{}, len(n.Tags))
		for _, tag := range n.Tags {
			t := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(tag), "#＃"))
			if t == "" {
				continue
			}
			if _, dup := seenInNode[t]; dup {
				continue
			}
			seenInNode[t] = struct{}{}
			counts[t]++
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

// WordCloudTerm represents an extracted, weighted keyword or concept for the 3D Inspiration Globe.
type WordCloudTerm struct {
	Word           string
	FilterTag      string // exact tag if Kind=="tag"
	Kind           string // "tag" | "concept"
	Theme          domain.OrbTheme
	IconKey        string
	Weight         float64
	NormWeight     float32 // 0.15 ~ 1.00 normalized relative to max term
	Percent        int     // proportional share summing cleanly to 100 across WordCloud
	Tier           int     // 0=core high-heat, 1=mid active, 2=long-tail whisper
	NodeCount      int
	EvidenceCount  int
	TopNodeID      string
	TopNodeTitle   string
	TopNodeSummary string
	NodeIDs        []string
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
	WordCloud       []WordCloudTerm
	Clues           []DashboardClue
}

// BuildDashboardDigest computes weighted keyword clusters, activity sparkline, and next clues from vault data.
func (s *OrganizeService) BuildDashboardDigest(nodes []domain.IdeaNode, evCounts map[string]int, insights []domain.SleepInsight) DashboardDigest {
	now := time.Now()
	digest := DashboardDigest{
		TotalNodes: len(nodes),
		Sparkline:  computeActivitySparkline(nodes, now),
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
	digest.WordCloud = buildWordCloudTerms(nodes, evCounts, now)

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

	for _, n := range nodes {
		byID[n.ID] = n

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
			"已记录 %d 篇灵感与 %d 条原始记录，重心聚焦在「%s」与「%s」（%d%%）。",
			len(nodes), digest.TotalEvidences, c0.Name, c1.Name, c0.Percent+c1.Percent,
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

func buildWordCloudTerms(nodes []domain.IdeaNode, evCounts map[string]int, now time.Time) []WordCloudTerm {
	if len(nodes) == 0 {
		return nil
	}

	type termAcc struct {
		term       WordCloudTerm
		isPrimary  bool
		topScore   float64
		nodeSeen   map[string]struct{}
	}

	accMap := make(map[string]*termAcc)
	addContribution := func(word, filterTag, kind string, n domain.IdeaNode, score, mult float64, evC int, isPrimary bool) {
		w := strings.TrimSpace(word)
		if w == "" {
			return
		}
		acc, ok := accMap[w]
		if !ok {
			acc = &termAcc{
				term: WordCloudTerm{
					Word:           w,
					FilterTag:      filterTag,
					Kind:           kind,
					Theme:          n.Theme,
					IconKey:        domain.ResolveIdeaIconKey(n),
					TopNodeID:      n.ID,
					TopNodeTitle:   n.Title,
					TopNodeSummary: n.ConversationalSummary,
				},
				topScore: score,
				nodeSeen: make(map[string]struct{}),
			}
			accMap[w] = acc
		}
		acc.term.Weight += score * mult
		if isPrimary {
			acc.isPrimary = true
		}
		if kind == "tag" && acc.term.Kind != "tag" {
			acc.term.Kind = "tag"
			acc.term.FilterTag = filterTag
		}
		if _, seen := acc.nodeSeen[n.ID]; !seen {
			acc.nodeSeen[n.ID] = struct{}{}
			acc.term.NodeIDs = append(acc.term.NodeIDs, n.ID)
			acc.term.NodeCount++
			acc.term.EvidenceCount += evC
		}
		if score > acc.topScore {
			acc.topScore = score
			acc.term.Theme = n.Theme
			acc.term.IconKey = domain.ResolveIdeaIconKey(n)
			acc.term.TopNodeID = n.ID
			acc.term.TopNodeTitle = n.Title
			acc.term.TopNodeSummary = n.ConversationalSummary
		}
	}

	nodeScores := make([]float64, len(nodes))
	for i, n := range nodes {
		evC := max(1, evCounts[n.ID])
		score := computeNodeWordScore(n, evC, now)
		nodeScores[i] = score

		seenNodeTags := make(map[string]struct{})
		primaryAssigned := false
		for _, rawTag := range n.Tags {
			tag := strings.TrimSpace(rawTag)
			if tag == "" {
				continue
			}
			if _, dup := seenNodeTags[tag]; dup {
				continue
			}
			seenNodeTags[tag] = struct{}{}
			if !primaryAssigned {
				addContribution(tag, tag, "tag", n, score, 1.00, evC, true)
				primaryAssigned = true
			} else {
				addContribution(tag, tag, "tag", n, score, 0.58, evC, false)
			}
		}
	}

	// Supplement with clean concept whispers if unique tags are fewer than 16.
	if len(accMap) < 16 {
		for i, n := range nodes {
			if len(accMap) >= 16 {
				break
			}
			for _, concept := range extractNodeConcepts(n) {
				if _, exists := accMap[concept]; exists {
					continue
				}
				evC := max(1, evCounts[n.ID])
				addContribution(concept, "", "concept", n, nodeScores[i], 0.26, evC, false)
				if len(accMap) >= 16 {
					break
				}
			}
		}
	}

	// Fallback if all nodes had empty tags and no concepts matched.
	if len(accMap) == 0 {
		n := nodes[0]
		addContribution("随手闪念", "", "concept", n, max(0.35, nodeScores[0]), 1.0, max(1, evCounts[n.ID]), true)
	}

	list := make([]*termAcc, 0, len(accMap))
	for _, acc := range accMap {
		list = append(list, acc)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].term.Weight != list[j].term.Weight {
			return list[i].term.Weight > list[j].term.Weight
		}
		if list[i].term.NodeCount != list[j].term.NodeCount {
			return list[i].term.NodeCount > list[j].term.NodeCount
		}
		return list[i].term.Word < list[j].term.Word
	})

	if len(list) > 18 {
		list = list[:18]
	}

	maxW := list[0].term.Weight
	totalW := 0.0
	for _, acc := range list {
		totalW += acc.term.Weight
	}

	out := make([]WordCloudTerm, len(list))
	rawPcts := make([]float64, len(list))
	pctSum := 0
	for i, acc := range list {
		norm := float32(1.0)
		if maxW > 0 {
			norm = float32(acc.term.Weight / maxW)
		}
		if norm < 0.15 {
			norm = 0.15
		} else if norm > 1.0 {
			norm = 1.0
		}
		if i == 0 {
			norm = 1.0
		}
		acc.term.NormWeight = norm

		raw := 0.0
		if totalW > 0 {
			raw = (acc.term.Weight / totalW) * 100.0
		}
		rawPcts[i] = raw
		p := max(1, int(math.Floor(raw)))
		acc.term.Percent = p
		pctSum += p

		switch {
		case i == 0 || (i < 4 && (acc.isPrimary || norm >= 0.72)):
			acc.term.Tier = 0
		case norm >= 0.42 || (acc.term.Kind == "tag" && i < 10):
			acc.term.Tier = 1
		default:
			acc.term.Tier = 2
		}

		out[i] = acc.term
	}

	reconcileWordCloudPercents(out, rawPcts, pctSum)
	return out
}

func computeNodeWordScore(n domain.IdeaNode, evCount int, now time.Time) float64 {
	recencyBonus := 0.0
	ts := n.UpdatedAt
	if ts.IsZero() {
		ts = n.CreatedAt
	}
	if !ts.IsZero() {
		ageHours := max(0.0, now.Sub(ts).Hours())
		recencyBonus = 0.14 * math.Exp(-ageHours/72.0)
	}
	return max(0.35, n.Weight) + 0.16*float64(evCount) + recencyBonus
}

func reconcileWordCloudPercents(out []WordCloudTerm, rawPcts []float64, pctSum int) {
	if len(out) == 0 {
		return
	}
	if pctSum < 100 {
		type remIdx struct {
			idx int
			rem float64
		}
		rems := make([]remIdx, len(out))
		for i := range out {
			rems[i] = remIdx{idx: i, rem: rawPcts[i] - float64(out[i].Percent)}
		}
		sort.Slice(rems, func(i, j int) bool {
			if rems[i].rem != rems[j].rem {
				return rems[i].rem > rems[j].rem
			}
			return rems[i].idx < rems[j].idx
		})
		need := 100 - pctSum
		for k := 0; k < need; k++ {
			out[rems[k%len(rems)].idx].Percent++
		}
		return
	}
	for pctSum > 100 {
		changed := false
		for i := len(out) - 1; i >= 0 && pctSum > 100; i-- {
			if out[i].Percent > 1 {
				out[i].Percent--
				pctSum--
				changed = true
			}
		}
		if !changed {
			break
		}
	}
}

var curatedConceptPhrases = []string{
	"郁金香", "蓝花瓶", "钴蓝玻璃", "手吹玻璃", "语音记忆球", "手表盲操",
	"单键录音", "噪点质感", "转移概率", "动态规划", "蒙特卡洛", "水彩插画",
	"手绘笔触", "阳台绿植", "晨间仪式", "跨端同步", "桌面美学", "随身手账",
	"光影折射", "声音笔记", "零视觉负担", "笔触颗粒", "薄荷绿植", "明信片",
}

var conceptStopwords = map[string]struct{}{
	"到底": {}, "什么": {}, "怎么": {}, "一个": {}, "一种": {}, "还是": {},
	"或者": {}, "为什么": {}, "如果": {}, "觉得": {}, "应该": {}, "可以": {},
	"自己": {}, "每天": {}, "早上": {}, "晚上": {}, "时候": {}, "东西": {},
}

func extractNodeConcepts(n domain.IdeaNode) []string {
	var out []string
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		if _, stop := conceptStopwords[s]; stop {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	hay := n.Title + " " + n.Visual.Caption + " " + n.ConversationalSummary
	for _, phrase := range curatedConceptPhrases {
		if strings.Contains(hay, phrase) {
			add(phrase)
		}
	}

	splitTokens := strings.FieldsFunc(n.Visual.Caption+" "+n.Title, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '·', '×', '：', ':', '、', '/', '|', '—', '–', '-', '，', '。', '？', '！', '「', '」', '（', '）', '“', '”', '"', '\'':
			return true
		default:
			return false
		}
	})
	for _, tok := range splitTokens {
		tok = strings.TrimSpace(tok)
		if strings.EqualFold(tok, "vs") {
			continue
		}
		rc := len([]rune(tok))
		if rc >= 2 && rc <= 5 {
			add(tok)
		}
	}
	return out
}
