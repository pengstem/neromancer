package service

import (
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/store"
)

func TestCaptureAutoMergeTulipAndRecall(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	// Fake DeepSeek API server simulating auto-merge decision into orb_tulip.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"finish_reason": "stop",
				"message": {
					"content": "{\"action\":\"merge\",\"target_node_id\":\"orb_tulip\",\"title\":\"用一个蓝色花瓶养一株带露水的郁金香\",\"conversational_summary\":\"把早起晒太阳和钴蓝花瓶的小念头合在一起了。\",\"tags\":[\"郁金香\",\"早晨晒太阳\"],\"theme\":\"coral\",\"weight\":0.96,\"visual\":{\"motif_type\":\"petals\",\"accent_hex\":\"#3B82F6\",\"caption\":\"蓝瓶晨光\",\"energy\":0.95},\"guiding_questions\":[\"今天去花店看一眼吗？\"]}"
				}
			}]
		}`))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	mm := media.NewManager(filepath.Join(tmpDir, "audio"), filepath.Join(tmpDir, "images"))
	aiClient := ai.NewClient(srv.URL, "sk-deepseek-demo", "deepseek-flash")
	capSvc := NewCaptureService(st, aiClient, mm)
	orgSvc := NewOrganizeService(st)

	outcome, err := capSvc.CaptureQuick(context.Background(), QuickCaptureInput{
		RawText:      "顺便再在蓝花瓶旁边放张明信片晒太阳",
		Modality:     "text",
		AutoOrganize: true,
		Scene: domain.SceneContext{
			Location:    "阳台",
			MoodWeather: "晴天",
		},
	})
	if err != nil {
		t.Fatalf("CaptureQuick failed: %v", err)
	}
	if outcome.MergedIntoID != "orb_tulip" {
		t.Fatalf("expected merge into orb_tulip, got %q (warning=%s)", outcome.MergedIntoID, outcome.AIWarning)
	}

	evs, err := st.ListEvidencesByNode("orb_tulip")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 {
		t.Fatalf("expected 3 raw evidences preserved on orb_tulip, got %d", len(evs))
	}

	// Ensure temporary shell node was cleaned up and total node count remained 4.
	nodes, err := st.ListNodes()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 nodes after merge cleanup, got %d", len(nodes))
	}

	hits, err := orgSvc.SearchRecall("明信片", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || hits[0].Node.ID != "orb_tulip" {
		t.Fatalf("expected recall hit for new raw evidence text, got %+v", hits)
	}
}

func TestShareCardPNGAndCapsuleExportImport(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	exportDir := filepath.Join(t.TempDir(), "exports")
	shareSvc := NewShareService(st, exportDir)

	pngPath, err := shareSvc.ExportPNGCard("orb_tulip")
	if err != nil {
		t.Fatalf("ExportPNGCard failed: %v", err)
	}
	f, err := os.Open(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("exported PNG is invalid: %v", err)
	}
	if decoded.Bounds().Dx() <= 0 || decoded.Bounds().Dy() <= 0 {
		t.Fatalf("exported PNG has empty bounds: %v", decoded.Bounds())
	}

	capPath, err := shareSvc.ExportCapsule("orb_tulip")
	if err != nil {
		t.Fatalf("ExportCapsule failed: %v", err)
	}
	rawCap, err := os.ReadFile(capPath)
	if err != nil {
		t.Fatal(err)
	}
	var capObj domain.OrbCapsule
	if err := json.Unmarshal(rawCap, &capObj); err != nil {
		t.Fatalf("invalid capsule json: %v", err)
	}

	imported, err := shareSvc.ImportCapsuleFile(capPath)
	if err != nil {
		t.Fatalf("ImportCapsuleFile failed: %v", err)
	}
	if imported.Title != capObj.Node.Title {
		t.Fatalf("unexpected imported title %q", imported.Title)
	}
}
