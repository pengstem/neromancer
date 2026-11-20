// Package store implements a pure-Go cross-platform ACID vault store for Neromancer
// with zero CGO/libc dependencies so Android, iOS, Desktop, and WebAssembly targets
// all compile and persist safely inside app sandboxes.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pengstem/neromancer/internal/domain"
)

var ErrNotFound = errors.New("record not found")

type vaultData struct {
	Version       string                   `json:"version"`
	Nodes         map[string]domain.IdeaNode     `json:"nodes"`
	Evidences     []domain.RawEvidence     `json:"evidences"`
	Snapshots     []domain.StageSnapshot   `json:"snapshots"`
	SleepInsights []domain.SleepInsight    `json:"sleep_insights"`
	RoamSlices    map[string]domain.RoamSlice `json:"roam_slices"`
	Settings      map[string]string        `json:"settings"`
}

func newVaultData() vaultData {
	return vaultData{
		Version:    "neromancer.vault.v1",
		Nodes:      make(map[string]domain.IdeaNode),
		RoamSlices: make(map[string]domain.RoamSlice),
		Settings:   make(map[string]string),
	}
}

// Store persists Neromancer inspirations, immutable raw evidences, snapshots, and settings.
// Writes are mutex-serialized and atomically flushed (temp file + rename).
type Store struct {
	mu   sync.RWMutex
	path string
	data vaultData
}

// Open opens (or creates) a pure-Go atomic JSON vault at vaultPath, or ":memory:" for tests.
func Open(vaultPath string) (*Store, error) {
	s := &Store{
		path: strings.TrimSpace(vaultPath),
		data: newVaultData(),
	}
	if s.path == "" || s.path == ":memory:" {
		s.path = ":memory:"
		return s, nil
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return nil, fmt.Errorf("prepare vault dir: %w", err)
	}

	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := s.flushLocked(); err != nil {
				return nil, err
			}
			return s, nil
		}
		return nil, fmt.Errorf("read vault %q: %w", s.path, err)
	}

	if len(strings.TrimSpace(string(raw))) > 0 {
		if err := json.Unmarshal(raw, &s.data); err != nil {
			return nil, fmt.Errorf("decode vault %q: %w", s.path, err)
		}
	}
	s.ensureMapsLocked()
	return s, nil
}

// Close flushes pending state and cleanly shuts down the store.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *Store) ensureMapsLocked() {
	if s.data.Nodes == nil {
		s.data.Nodes = make(map[string]domain.IdeaNode)
	}
	if s.data.RoamSlices == nil {
		s.data.RoamSlices = make(map[string]domain.RoamSlice)
	}
	if s.data.Settings == nil {
		s.data.Settings = make(map[string]string)
	}
}

func (s *Store) flushLocked() error {
	if s.path == "" || s.path == ":memory:" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// ListNodes returns all inspiration nodes ordered by weight and recency.
func (s *Store) ListNodes() ([]domain.IdeaNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]domain.IdeaNode, 0, len(s.data.Nodes))
	for _, n := range s.data.Nodes {
		nodes = append(nodes, cloneNode(n))
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Weight != nodes[j].Weight {
			return nodes[i].Weight > nodes[j].Weight
		}
		if !nodes[i].UpdatedAt.Equal(nodes[j].UpdatedAt) {
			return nodes[i].UpdatedAt.After(nodes[j].UpdatedAt)
		}
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, nil
}

// GetNode fetches a single IdeaNode by ID.
func (s *Store) GetNode(id string) (domain.IdeaNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.data.Nodes[id]
	if !ok {
		return domain.IdeaNode{}, ErrNotFound
	}
	return cloneNode(n), nil
}

// UpsertNode inserts or updates an IdeaNode cleanly.
func (s *Store) UpsertNode(node domain.IdeaNode) error {
	if err := node.Validate(); err != nil {
		return err
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}
	if node.UpdatedAt.IsZero() {
		node.UpdatedAt = time.Now()
	}
	if node.Weight <= 0 {
		node.Weight = 0.5
	}
	if node.Stage == "" {
		node.Stage = domain.StageSpark
	}
	node.Theme = domain.NormalizeTheme(string(node.Theme))

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMapsLocked()
	s.data.Nodes[node.ID] = cloneNode(node)
	return s.flushLocked()
}

// DeleteNode removes an obsolete provisional IdeaNode shell (raw evidences should be reassigned first).
func (s *Store) DeleteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Nodes, id)
	return s.flushLocked()
}

// UpdateNodeStage transitions an IdeaNode's lifecycle stage and records an immutable StageSnapshot.
func (s *Store) UpdateNodeStage(id string, stage domain.LifecycleStage, note string) (domain.IdeaNode, domain.StageSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.data.Nodes[id]
	if !ok {
		return domain.IdeaNode{}, domain.StageSnapshot{}, ErrNotFound
	}
	now := time.Now()
	node.Stage = stage
	node.UpdatedAt = now
	s.data.Nodes[id] = cloneNode(node)

	snap := domain.StageSnapshot{
		ID:        fmt.Sprintf("snap_%d", now.UnixNano()),
		NodeID:    node.ID,
		Stage:     stage,
		Title:     node.Title,
		Summary:   node.ConversationalSummary,
		Note:      note,
		CreatedAt: now,
	}
	s.data.Snapshots = append(s.data.Snapshots, snap)
	if err := s.flushLocked(); err != nil {
		return domain.IdeaNode{}, domain.StageSnapshot{}, err
	}
	return cloneNode(node), snap, nil
}

// AddRawEvidence appends an immutable RawEvidence entry. Raw user captures are never overwritten.
func (s *Store) AddRawEvidence(ev domain.RawEvidence) error {
	if strings.TrimSpace(ev.ID) == "" {
		return fmt.Errorf("raw evidence id cannot be empty")
	}
	if ev.CapturedAt.IsZero() {
		ev.CapturedAt = time.Now()
	}
	if ev.Modality == "" {
		ev.Modality = "text"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.data.Evidences {
		if existing.ID == ev.ID {
			return nil // Append-only immutability: ignore duplicate ID writes
		}
	}
	s.data.Evidences = append(s.data.Evidences, ev)
	return s.flushLocked()
}

// ReassignEvidenceNode links an existing immutable RawEvidence to a merged IdeaNode without modifying its content.
func (s *Store) ReassignEvidenceNode(evidenceID, targetNodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Evidences {
		if s.data.Evidences[i].ID == evidenceID {
			s.data.Evidences[i].NodeID = targetNodeID
			return s.flushLocked()
		}
	}
	return ErrNotFound
}

// ListEvidencesByNode returns all original captures attached to nodeID, oldest first.
func (s *Store) ListEvidencesByNode(nodeID string) ([]domain.RawEvidence, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []domain.RawEvidence
	for _, ev := range s.data.Evidences {
		if ev.NodeID == nodeID {
			out = append(out, ev)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CapturedAt.Before(out[j].CapturedAt)
	})
	return out, nil
}

// CountEvidencesByNode returns a map of node_id -> count of immutable raw evidences.
func (s *Store) CountEvidencesByNode() (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for _, ev := range s.data.Evidences {
		counts[ev.NodeID]++
	}
	return counts, nil
}

// AddStageSnapshot stores a lifecycle snapshot for an IdeaNode.
func (s *Store) AddStageSnapshot(snap domain.StageSnapshot) error {
	if snap.CreatedAt.IsZero() {
		snap.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.data.Snapshots {
		if s.data.Snapshots[i].ID == snap.ID {
			s.data.Snapshots[i] = snap
			return s.flushLocked()
		}
	}
	s.data.Snapshots = append(s.data.Snapshots, snap)
	return s.flushLocked()
}

// ListSnapshotsByNode returns chronological stage snapshots for a node.
func (s *Store) ListSnapshotsByNode(nodeID string) ([]domain.StageSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []domain.StageSnapshot
	for _, snap := range s.data.Snapshots {
		if snap.NodeID == nodeID {
			out = append(out, snap)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// SearchAssociativeRecall surfaces past inspirations matching keywords in the active thought/query.
func (s *Store) SearchAssociativeRecall(query string, limit int) ([]domain.RecallMatch, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 4
	}
	nodes, err := s.ListNodes()
	if err != nil {
		return nil, err
	}
	counts, err := s.CountEvidencesByNode()
	if err != nil {
		return nil, err
	}

	tokens := splitKeywords(q)
	var matches []domain.RecallMatch
	for _, node := range nodes {
		reason, matchedText := matchNode(node, tokens)
		if reason == "" {
			evs, _ := s.ListEvidencesByNode(node.ID)
			for _, ev := range evs {
				if containsAnyToken(ev.RawText, tokens) || containsAnyToken(ev.Scene.Location, tokens) {
					reason = "原记录里提过"
					matchedText = ev.RawText
					break
				}
			}
		}
		if reason != "" {
			matches = append(matches, domain.RecallMatch{
				Node:          node,
				MatchedText:   matchedText,
				MatchReason:   reason,
				EvidenceCount: counts[node.ID],
			})
			if len(matches) >= limit {
				break
			}
		}
	}
	return matches, nil
}

// SaveSleepInsight inserts or updates a sleep incubation insight.
func (s *Store) SaveSleepInsight(insight domain.SleepInsight) error {
	if insight.CreatedAt.IsZero() {
		insight.CreatedAt = time.Now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.SleepInsights {
		if s.data.SleepInsights[i].ID == insight.ID {
			s.data.SleepInsights[i] = insight
			return s.flushLocked()
		}
	}
	s.data.SleepInsights = append(s.data.SleepInsights, insight)
	return s.flushLocked()
}

// ListActiveSleepInsights returns un-dismissed sleep incubation insights.
func (s *Store) ListActiveSleepInsights() ([]domain.SleepInsight, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []domain.SleepInsight
	for _, item := range s.data.SleepInsights {
		if !item.Dismissed {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// DismissSleepInsight marks a sleep insight as reviewed.
func (s *Store) DismissSleepInsight(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.SleepInsights {
		if s.data.SleepInsights[i].ID == id {
			s.data.SleepInsights[i].Dismissed = true
			return s.flushLocked()
		}
	}
	return nil
}

// ListRoamSlices returns read-only community inspiration slices for exploration and collision.
func (s *Store) ListRoamSlices() ([]domain.RoamSlice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]domain.RoamSlice, 0, len(s.data.RoamSlices))
	for _, rs := range s.data.RoamSlices {
		out = append(out, rs)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// UpsertRoamSlice saves a read-only roaming slice.
func (s *Store) UpsertRoamSlice(rs domain.RoamSlice) error {
	rs.Theme = domain.NormalizeTheme(string(rs.Theme))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMapsLocked()
	s.data.RoamSlices[rs.ID] = rs
	return s.flushLocked()
}

// GetSetting reads a persisted UI/API setting.
func (s *Store) GetSetting(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Settings[key], nil
}

// SetSetting persists a setting key/value pair.
func (s *Store) SetSetting(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureMapsLocked()
	s.data.Settings[key] = value
	return s.flushLocked()
}

// ExportOrbCapsule bundles an IdeaNode with all of its immutable RawEvidences and StageSnapshots.
func (s *Store) ExportOrbCapsule(nodeID string) (domain.OrbCapsule, error) {
	node, err := s.GetNode(nodeID)
	if err != nil {
		return domain.OrbCapsule{}, err
	}
	evs, err := s.ListEvidencesByNode(nodeID)
	if err != nil {
		return domain.OrbCapsule{}, err
	}
	snaps, err := s.ListSnapshotsByNode(nodeID)
	if err != nil {
		return domain.OrbCapsule{}, err
	}
	return domain.OrbCapsule{
		Version:    "neromancer.orb.v1",
		ExportedAt: time.Now(),
		Node:       node,
		Evidences:  evs,
		Snapshots:  snaps,
	}, nil
}

// ImportOrbCapsule imports a portable .orb.json capsule into the local inspiration vault.
func (s *Store) ImportOrbCapsule(capsule domain.OrbCapsule) (domain.IdeaNode, error) {
	node := capsule.Node
	if strings.TrimSpace(node.ID) == "" {
		node.ID = fmt.Sprintf("orb_imp_%d", time.Now().UnixNano())
	} else {
		if _, err := s.GetNode(node.ID); err == nil {
			node.ID = fmt.Sprintf("%s_imp_%d", node.ID, time.Now().Unix()%100000)
		}
	}
	node.UpdatedAt = time.Now()
	if err := s.UpsertNode(node); err != nil {
		return domain.IdeaNode{}, err
	}
	for i, ev := range capsule.Evidences {
		ev.ID = fmt.Sprintf("%s_ev_%d_%d", node.ID, i, time.Now().UnixNano()%10000)
		ev.NodeID = node.ID
		if err := s.AddRawEvidence(ev); err != nil {
			return domain.IdeaNode{}, err
		}
	}
	for i, snap := range capsule.Snapshots {
		snap.ID = fmt.Sprintf("%s_snap_%d_%d", node.ID, i, time.Now().UnixNano()%10000)
		snap.NodeID = node.ID
		if err := s.AddStageSnapshot(snap); err != nil {
			return domain.IdeaNode{}, err
		}
	}
	return node, nil
}

func cloneNode(n domain.IdeaNode) domain.IdeaNode {
	n.Tags = slices.Clone(n.Tags)
	n.GuidingQuestions = slices.Clone(n.GuidingQuestions)
	n.RelatedIDs = slices.Clone(n.RelatedIDs)
	return n
}

func splitKeywords(query string) []string {
	raw := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return r == ' ' || r == ',' || r == '，' || r == '。' || r == '、' || r == '\t' || r == '\n'
	})
	var out []string
	for _, tok := range raw {
		tok = strings.TrimSpace(tok)
		if len([]rune(tok)) >= 1 {
			out = append(out, tok)
		}
	}
	if len(out) == 1 {
		runes := []rune(out[0])
		if len(runes) >= 3 {
			for i := 0; i <= len(runes)-2; i++ {
				out = append(out, string(runes[i:i+2]))
			}
		}
	}
	return out
}

func containsAnyToken(text string, tokens []string) bool {
	lower := strings.ToLower(text)
	for _, tok := range tokens {
		if tok != "" && strings.Contains(lower, tok) {
			return true
		}
	}
	return false
}

func matchNode(node domain.IdeaNode, tokens []string) (reason, excerpt string) {
	if containsAnyToken(node.Title, tokens) {
		return "标题提到", node.Title
	}
	for _, tag := range node.Tags {
		if containsAnyToken(tag, tokens) {
			return "标签 #" + tag, node.Title
		}
	}
	if containsAnyToken(node.ConversationalSummary, tokens) {
		return "内容提到", node.ConversationalSummary
	}
	return "", ""
}
