package ui

import (
	"context"
	"testing"
	"time"

	"github.com/pengstem/neromancer/internal/service"
)

func TestObsidianPulseGraphAndFileExplorerInteractions(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveGraph

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	// 1. Verify "漫游" is absent and both Obsidian Graph stats + AI File Explorer folders render.
	if tt.HasText("漫游") {
		t.Fatalf("expected '漫游' to be completely removed, got texts=%v", tt.Texts())
	}
	if !tt.HasText("AI 归档目录") || !tt.HasText("器物与光影美学") || !tt.HasText("算法与系统直觉") {
		t.Fatalf("missing expected AI folders in Pulse view, texts=%v", tt.Texts())
	}

	pgBefore := app.ensurePulseGraph()
	initialVertices := len(pgBefore.Nodes)
	initialEdges := len(pgBefore.Edges)
	if initialVertices < 8 || initialEdges < 6 {
		t.Fatalf("expected populated force-directed graph, got %d vertices and %d edges", initialVertices, initialEdges)
	}

	// 2. Folder collapse & expand in AI File Explorer.
	if !tt.HasText("用不完美噪点做温暖的手绘感着色器") {
		t.Fatalf("expected note inside expanded folder '器物与光影美学'")
	}
	if err := tt.Click("器物与光影美学"); err != nil {
		t.Fatalf("click folder header to collapse: %v", err)
	}
	settleAudit(tt)
	if !app.Pulse.CollapsedFolders["器物与光影美学"] {
		t.Fatalf("expected folder '器物与光影美学' to be collapsed")
	}
	if tt.HasText("用不完美噪点做温暖的手绘感着色器") {
		t.Fatalf("expected collapsed folder's child files to hide")
	}

	// 3. Selecting a node automatically re-expands its folder in AI File Explorer.
	app.SelectNode("orb_shader")
	settleAudit(tt)
	if app.Pulse.CollapsedFolders["器物与光影美学"] {
		t.Fatalf("expected SelectNode to auto-expand node's folder")
	}

	// 4. Clicking a file row in AI File Explorer selects it and opens Detail Sheet on phone.
	if err := tt.Click("有模型与免模型强化学习到底差在哪一步？"); err != nil {
		t.Fatalf("click RL file row in AI File Explorer: %v", err)
	}
	settleAudit(tt)
	if app.SelectedID != "orb_rl_prose" || !app.DetailSheetOpen {
		t.Fatalf("expected orb_rl_prose selected & DetailSheetOpen=true, got id=%s open=%v", app.SelectedID, app.DetailSheetOpen)
	}
	app.DetailSheetOpen = false
	settleAudit(tt)

	// 5. Dragging on the Graph Canvas pans the constellation WITHOUT triggering SpaceDeck floor transition!
	tt.FingerDown(206, 200)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(266, 240)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(266, 240)
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("dragging inside Obsidian Graph should pan graph, not pull SpaceDeck to Dashboard")
	}
	if app.Pulse.PanX == 0 && app.Pulse.PanY == 0 {
		t.Fatalf("expected non-zero PanX/PanY after dragging graph canvas")
	}

	// 6. Capturing a new related inspiration automatically adds new graph vertices & connects edges!
	out, err := app.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:  "用钴蓝玻璃杯在书桌阳光下拍水波焦散",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick failed: %v", err)
	}
	if err := app.ReloadVault(); err != nil {
		t.Fatalf("ReloadVault failed: %v", err)
	}
	settleAudit(tt)

	pgAfter := app.ensurePulseGraph()
	if len(pgAfter.Nodes) <= initialVertices || len(pgAfter.Edges) <= initialEdges {
		t.Fatalf("expected graph to grow naturally on new capture: before=(%d,%d) after=(%d,%d)",
			initialVertices, initialEdges, len(pgAfter.Nodes), len(pgAfter.Edges))
	}
	newIdx, ok := pgAfter.NoteIndexByID[out.Node.ID]
	if !ok || pgAfter.Nodes[newIdx].Degree == 0 {
		t.Fatalf("expected newly captured inspiration to be present and automatically connected, ok=%v node=%+v", ok, pgAfter.Nodes[newIdx])
	}
}
