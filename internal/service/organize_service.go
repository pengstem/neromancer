package service

import (
	"fmt"
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
