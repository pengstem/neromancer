package service

import (
	"context"
	"encoding/json"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestDashboardDigestAndAutoIconOnCapture(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	mm := media.NewManager(filepath.Join(tmpDir, "audio"), filepath.Join(tmpDir, "images"))
	aiClient := ai.NewClient("https://api.deepseek.com", "", "deepseek-flash")
	capSvc := NewCaptureService(st, aiClient, mm)
	orgSvc := NewOrganizeService(st)

	// CaptureQuick should synchronously assign semantic SVG icon key on local save.
	out, err := capSvc.CaptureQuick(context.Background(), QuickCaptureInput{
		RawText:  "在阳台种一盆薄荷",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick failed: %v", err)
	}
	if out.Node.Visual.MotifType != domain.IconKeyBotanical {
		t.Fatalf("expected auto-assigned botanical icon, got %q", out.Node.Visual.MotifType)
	}

	nodes, counts, _, _, err := orgSvc.ListVault()
	if err != nil {
		t.Fatal(err)
	}
	insights, err := st.ListActiveSleepInsights()
	if err != nil {
		t.Fatal(err)
	}

	digest := orgSvc.BuildDashboardDigest(nodes, counts, insights)
	if digest.PersonaHeadline == "" || digest.PersonaSummary == "" {
		t.Fatalf("expected non-empty persona headline/summary, got %+v", digest)
	}
	if strings.Contains(digest.PersonaSummary, "已推进") {
		t.Fatalf("expected PersonaSummary without stage progress copy, got %q", digest.PersonaSummary)
	}
	if digest.TotalNodes != len(nodes) {
		t.Fatalf("expected TotalNodes=%d, got %d", len(nodes), digest.TotalNodes)
	}
	if len(digest.Sparkline) != 6 {
		t.Fatalf("expected 6-point sparkline, got %d", len(digest.Sparkline))
	}
	if len(digest.Clusters) != 4 {
		t.Fatalf("expected 4 keyword clusters, got %d", len(digest.Clusters))
	}
	pctSum := 0
	for _, cl := range digest.Clusters {
		pctSum += cl.Percent
		if cl.IconKey == "" || cl.Name == "" {
			t.Fatalf("cluster missing icon or name: %+v", cl)
		}
	}
	if pctSum != 100 {
		t.Fatalf("expected cluster percentages to sum to 100, got %d", pctSum)
	}
	if len(digest.Clues) != 2 {
		t.Fatalf("expected 2 actionable clues (sleep + question), got %+v", digest.Clues)
	}
	if len(digest.WordCloud) == 0 || len(digest.WordCloud) > 18 {
		t.Fatalf("expected 1..18 WordCloud terms, got %d", len(digest.WordCloud))
	}
	wcPctSum := 0
	for i, term := range digest.WordCloud {
		if term.Word == "" || term.TopNodeID == "" || term.TopNodeTitle == "" {
			t.Fatalf("word cloud term[%d] missing word/top node metadata: %+v", i, term)
		}
		if math.IsNaN(term.Weight) || math.IsInf(term.Weight, 0) || math.IsNaN(float64(term.NormWeight)) {
			t.Fatalf("word cloud term[%d] contains NaN/Inf: %+v", i, term)
		}
		if term.NormWeight < 0.15 || term.NormWeight > 1.0 {
			t.Fatalf("word cloud term[%d] NormWeight out of [0.15, 1.0]: %v", i, term.NormWeight)
		}
		if i == 0 && term.NormWeight != 1.0 {
			t.Fatalf("expected top word cloud term NormWeight=1.0, got %v", term.NormWeight)
		}
		if i > 0 && digest.WordCloud[i-1].Weight < term.Weight {
			t.Fatalf("word cloud not sorted by weight descending at %d: %v < %v", i, digest.WordCloud[i-1].Weight, term.Weight)
		}
		if term.Percent < 1 {
			t.Fatalf("expected term[%d] Percent >= 1, got %d", i, term.Percent)
		}
		wcPctSum += term.Percent
	}
	if wcPctSum != 100 {
		t.Fatalf("expected WordCloud percentages to sum to 100, got %d", wcPctSum)
	}
}

func TestPulseGraphAutoLinkAndAIFolderTree(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	mm := media.NewManager(filepath.Join(tmpDir, "audio"), filepath.Join(tmpDir, "images"))
	aiClient := ai.NewClient("https://api.deepseek.com", "", "deepseek-flash")
	capSvc := NewCaptureService(st, aiClient, mm)
	orgSvc := NewOrganizeService(st)

	// Capturing a related botanical/light idea offline should immediately assign folder & auto-link to orb_tulip!
	out, err := capSvc.CaptureQuick(context.Background(), QuickCaptureInput{
		RawText:  "在阳台玻璃瓶旁边再养一株薄荷看清晨光影",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick failed: %v", err)
	}
	if out.Node.Folder != "器物与光影美学" {
		t.Fatalf("expected auto-assigned folder '器物与光影美学', got %q", out.Node.Folder)
	}
	if len(out.Node.RelatedIDs) == 0 {
		t.Fatalf("expected newly captured related idea to auto-link to existing nodes")
	}

	nodes, _, _, _, err := orgSvc.ListVault()
	if err != nil {
		t.Fatal(err)
	}
	evs, err := orgSvc.ListAllEvidences()
	if err != nil {
		t.Fatal(err)
	}
	insights, _ := st.ListActiveSleepInsights()

	pg := orgSvc.BuildPulseGraph(nodes, evs, insights, PulseGraphOptions{
		ShowTags:      true,
		ShowEvidences: true,
	})
	if len(pg.Nodes) < len(nodes)+len(evs) {
		t.Fatalf("expected note + tag + evidence vertices in PulseGraph, got %d", len(pg.Nodes))
	}
	if len(pg.Edges) == 0 {
		t.Fatalf("expected connected edges in PulseGraph")
	}
	newIdx, ok := pg.NoteIndexByID[out.Node.ID]
	if !ok {
		t.Fatalf("newly captured node missing from PulseGraph")
	}
	tulipIdx := pg.NoteIndexByID["orb_tulip"]
	if !pg.NeighborsByIdx[newIdx][tulipIdx] {
		t.Fatalf("expected newly captured idea to have a direct edge to orb_tulip in PulseGraph")
	}
	if len(pg.Folders) < 3 {
		t.Fatalf("expected AI-summarized folder list with >=3 folders, got %d", len(pg.Folders))
	}
	for _, nd := range pg.Nodes {
		if nd.X < 0.05 || nd.X > 0.95 || nd.Y < 0.05 || nd.Y > 0.95 || math.IsNaN(nd.X) || math.IsNaN(nd.Y) {
			t.Fatalf("graph node %s out of normalized bounds: (%v, %v)", nd.ID, nd.X, nd.Y)
		}
	}
}

func TestWordCloudEdgeCases(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	orgSvc := NewOrganizeService(st)

	emptyDigest := orgSvc.BuildDashboardDigest(nil, nil, nil)
	if len(emptyDigest.WordCloud) != 0 {
		t.Fatalf("expected empty WordCloud for empty vault, got %+v", emptyDigest.WordCloud)
	}

	singleNode := []domain.IdeaNode{{
		ID:                    "single_1",
		Title:                 "清晨手冲咖啡实验",
		ConversationalSummary: "试一下浅烘耶加雪菲用 92 度水温。",
		Tags:                  []string{"手冲咖啡"},
		Theme:                 domain.ThemeAmber,
		Weight:                0.8,
	}}
	singleDigest := orgSvc.BuildDashboardDigest(singleNode, map[string]int{"single_1": 1}, nil)
	if len(singleDigest.WordCloud) == 0 {
		t.Fatalf("expected non-empty WordCloud for 1-node vault")
	}
	pctSum := 0
	for i, term := range singleDigest.WordCloud {
		if math.IsNaN(term.Weight) || math.IsNaN(float64(term.NormWeight)) {
			t.Fatalf("single vault term[%d] has NaN: %+v", i, term)
		}
		if term.TopNodeID != "single_1" || term.TopNodeTitle != "清晨手冲咖啡实验" {
			t.Fatalf("single vault term[%d] wrong top node: %+v", i, term)
		}
		pctSum += term.Percent
	}
	if pctSum != 100 {
		t.Fatalf("expected single-node WordCloud percentages to sum to 100, got %d", pctSum)
	}
}
