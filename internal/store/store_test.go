package store

import (
	"testing"
	"time"

	"github.com/pengstem/neromancer/internal/domain"
)

func TestStoreSeedAndRawEvidencePreservation(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	defer s.Close()

	if err := s.SeedIfEmpty(); err != nil {
		t.Fatalf("seed db: %v", err)
	}
	// Idempotency check
	if err := s.SeedIfEmpty(); err != nil {
		t.Fatalf("second seed should be idempotent: %v", err)
	}

	nodes, err := s.ListNodes()
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 seeded nodes, got %d", len(nodes))
	}

	tulip, err := s.GetNode("orb_tulip")
	if err != nil {
		t.Fatalf("get tulip node: %v", err)
	}
	if tulip.Title != "用一个蓝色花瓶养一株郁金香" {
		t.Fatalf("unexpected tulip title: %q", tulip.Title)
	}

	evs, err := s.ListEvidencesByNode("orb_tulip")
	if err != nil {
		t.Fatalf("list tulip evidences: %v", err)
	}
	if len(evs) != 2 {
		t.Fatalf("expected 2 immutable raw evidences for tulip node, got %d", len(evs))
	}
}

func TestAssociativeRecallAndCapsuleRoundTrip(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer s.Close()

	if err := s.SeedIfEmpty(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	matches, err := s.SearchAssociativeRecall("花瓶", 4)
	if err != nil {
		t.Fatalf("recall search: %v", err)
	}
	if len(matches) == 0 || matches[0].Node.ID != "orb_tulip" {
		t.Fatalf("expected recall hit for orb_tulip, got %+v", matches)
	}

	capsule, err := s.ExportOrbCapsule("orb_tulip")
	if err != nil {
		t.Fatalf("export capsule: %v", err)
	}
	if len(capsule.Evidences) != 2 {
		t.Fatalf("expected 2 evidences in capsule, got %d", len(capsule.Evidences))
	}

	imported, err := s.ImportOrbCapsule(capsule)
	if err != nil {
		t.Fatalf("import capsule: %v", err)
	}
	if imported.ID == "orb_tulip" {
		t.Fatalf("imported duplicate capsule should get a distinct ID")
	}
	impEvs, err := s.ListEvidencesByNode(imported.ID)
	if err != nil {
		t.Fatalf("list imported evidences: %v", err)
	}
	if len(impEvs) != 2 {
		t.Fatalf("expected 2 evidences copied into imported node, got %d", len(impEvs))
	}

	// Verify append-only evidence addition & reassignment.
	newEv := domain.RawEvidence{
		ID:         "ev_extra_1",
		NodeID:     "orb_watch",
		Modality:   "text",
		RawText:    "补充一条蓝瓶摆拍角度",
		CapturedAt: time.Now(),
	}
	if err := s.AddRawEvidence(newEv); err != nil {
		t.Fatalf("add raw evidence: %v", err)
	}
	if err := s.ReassignEvidenceNode("ev_extra_1", "orb_tulip"); err != nil {
		t.Fatalf("reassign evidence: %v", err)
	}
	evs, _ := s.ListEvidencesByNode("orb_tulip")
	if len(evs) != 3 {
		t.Fatalf("expected 3 evidences on tulip after reassignment, got %d", len(evs))
	}
}
