package ui

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/service"
)

func TestObsidianPulseGraphAndFileExplorerInteractions(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveGraph

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	// 1. Verify "漫游" & cluttered graph/folder stat hints are absent and multi-island Obsidian Graph + AI File Explorer folders render.
	if tt.HasText("漫游") {
		t.Fatalf("expected '漫游' to be completely removed, got texts=%v", tt.Texts())
	}
	if !tt.HasText("器物与光影美学") || !tt.HasText("算法与系统直觉") {
		t.Fatalf("missing expected AI folders in Pulse view, texts=%v", tt.Texts())
	}

	pgBefore := app.ensurePulseGraph(true)
	if pgBefore.IslandCount != 3 {
		t.Fatalf("expected 3 independent islands on seed vault, got %d", pgBefore.IslandCount)
	}
	initialVertices := len(pgBefore.Nodes)
	initialEdges := len(pgBefore.Edges)

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

	// 5. Dragging blank space on the Graph Canvas pans the constellation WITHOUT pulling SpaceDeck!
	tt.FingerDown(30, 110)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(95, 145)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(95, 145)
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("dragging inside Obsidian Graph should pan graph, not pull SpaceDeck to Dashboard")
	}
	if app.Pulse.PanX == 0 && app.Pulse.PanY == 0 {
		t.Fatalf("expected non-zero PanX/PanY after dragging blank graph canvas")
	}

	// 6. Capturing a related inspiration connects to its island; capturing an unrelated inspiration forms a new island!
	relOut, err := app.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:  "用钴蓝玻璃杯在书桌阳光下拍水波焦散",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick related failed: %v", err)
	}
	unrelOut, err := app.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:  "周末试做一碗葱油拌面",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick unrelated failed: %v", err)
	}
	if err := app.ReloadVault(); err != nil {
		t.Fatalf("ReloadVault failed: %v", err)
	}
	settleAudit(tt)

	pgAfter := app.ensurePulseGraph(true)
	if len(pgAfter.Nodes) <= initialVertices || len(pgAfter.Edges) <= initialEdges {
		t.Fatalf("expected graph to grow naturally on new capture: before=(%d,%d) after=(%d,%d)",
			initialVertices, initialEdges, len(pgAfter.Nodes), len(pgAfter.Edges))
	}
	if pgAfter.IslandCount != 4 {
		t.Fatalf("expected 4 separate islands after adding 1 related + 1 unrelated thought, got %d", pgAfter.IslandCount)
	}
	relIdx := pgAfter.NoteIndexByID[relOut.Node.ID]
	unrelIdx := pgAfter.NoteIndexByID[unrelOut.Node.ID]
	if pgAfter.Nodes[relIdx].Degree == 0 {
		t.Fatalf("expected related thought to connect into existing island")
	}
	if pgAfter.Nodes[unrelIdx].Degree != 0 {
		t.Fatalf("expected unrelated thought to remain an independent 0-degree island, got degree=%d", pgAfter.Nodes[unrelIdx].Degree)
	}
}

func TestObsidianPulseSpringBloomAndElasticIslandDrag(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveGraph

	// Enable live motion (ReduceMotion: false) to verify Obsidian bloom, breathing, and elastic island drag!
	tt := ui.NewTesterWith(app.RootView, 412, 892, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: false},
	})
	tt.Frame()

	tulipIdx := app.Pulse.Graph.NoteIndexByID["orb_tulip"]
	shaderIdx := app.Pulse.Graph.NoteIndexByID["orb_shader"]
	rlIdx := app.Pulse.Graph.NoteIndexByID["orb_rl_prose"]

	// 1. Capturing a new node mid-session starts in soft bloom state (Appear < 0.95) and smoothly settles to 1.0.
	newOut, err := app.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:  "深夜听黑胶唱片时的唱针颗粒声",
		Modality: "text",
	})
	if err != nil {
		t.Fatalf("CaptureQuick failed: %v", err)
	}
	_ = app.ReloadVault()
	tt.Frame()

	newIdx := app.Pulse.Graph.NoteIndexByID[newOut.Node.ID]
	if app.Pulse.SimNodes[newIdx].Appear >= 0.95 {
		t.Fatalf("expected newly spawned node to start in soft bloom (<0.95), got %v", app.Pulse.SimNodes[newIdx].Appear)
	}
	for i := 0; i < 60 && tt.FramePending(); i++ {
		tt.Advance(25 * time.Millisecond)
	}
	if app.Pulse.SimNodes[newIdx].Appear < 0.98 {
		t.Fatalf("expected bloom Appear to settle near 1.0, got %v", app.Pulse.SimNodes[newIdx].Appear)
	}
	if app.Pulse.BreathTime <= 0 {
		t.Fatalf("expected BreathTime to advance in live motion mode")
	}

	tulipIdx = app.Pulse.Graph.NoteIndexByID["orb_tulip"]
	shaderIdx = app.Pulse.Graph.NoteIndexByID["orb_shader"]
	rlIdx = app.Pulse.Graph.NoteIndexByID["orb_rl_prose"]

	// 2. Real finger-drag on orb_tulip's exact rendered vertex elastically tugs connected orb_shader (same island)
	// while leaving independent island orb_rl_prose undisturbed!
	tulipWinX := app.Pulse.LastStageBounds.X + app.Pulse.LastProj[tulipIdx].SX
	tulipWinY := app.Pulse.LastStageBounds.Y + app.Pulse.LastProj[tulipIdx].SY

	shaderBeforeX := app.Pulse.SimNodes[shaderIdx].X
	rlBeforeX := app.Pulse.SimNodes[rlIdx].X

	tt.FingerDown(tulipWinX, tulipWinY)
	if app.Pulse.DraggingNodeIdx != tulipIdx {
		t.Fatalf("expected FingerDown on orb_tulip dot to grab tulipIdx=%d, got %d", tulipIdx, app.Pulse.DraggingNodeIdx)
	}
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(tulipWinX-70, tulipWinY-10)
	for i := 0; i < 6; i++ {
		tt.Advance(25 * time.Millisecond)
	}

	shaderDelta := math.Abs(app.Pulse.SimNodes[shaderIdx].X - shaderBeforeX)
	rlDelta := math.Abs(app.Pulse.SimNodes[rlIdx].X - rlBeforeX)
	if shaderDelta < 0.015 {
		t.Fatalf("expected connected neighbor orb_shader to elastically follow finger-dragged orb_tulip, delta=%v", shaderDelta)
	}
	if rlDelta > shaderDelta*0.15 {
		t.Fatalf("expected independent island orb_rl_prose to stay undisturbed while pulling tulip island: rlDelta=%v shaderDelta=%v", rlDelta, shaderDelta)
	}

	// Release finger and verify island rebounds smoothly.
	tt.FingerUp(tulipWinX-70, tulipWinY-10)
	for i := 0; i < 10; i++ {
		tt.Advance(40 * time.Millisecond)
	}
}

func TestPulseGraphNeverOverflowsAboveOrBelowStage(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveGraph
	app.ArchiveSearchOpen = true
	app.Pulse.ShowTags = true

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	stageRect := app.Pulse.LastStageBounds
	if stageRect.Y < 40 || stageRect.H < 150 {
		t.Fatalf("unexpected stage bounds: %+v", stageRect)
	}
	baseSrc := tt.Image()
	if baseSrc == nil {
		t.Fatalf("expected non-nil tester framebuffer image")
	}
	b := baseSrc.Bounds()
	scale := float32(b.Dx()) / 412.0
	if scale <= 0 {
		scale = 1
	}

	// Copy baseline pixels so subsequent frames cannot mutate baseSrc in place.
	baseAbove := make([][4]uint8, b.Dx()*b.Dy())
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := baseSrc.RGBAAt(x, y)
			baseAbove[y*b.Dx()+x] = [4]uint8{c.R, c.G, c.B, c.A}
		}
	}

	// 1. Pan strongly upward & zoom in so vertices and edges cross well above stage top (SY < 0).
	app.Pulse.Zoom = 2.4
	app.Pulse.PanY = -150
	settleAudit(tt)

	hasAboveTop := false
	for _, pv := range app.Pulse.LastProj {
		if pv.SY < -10 {
			hasAboveTop = true
			break
		}
	}
	if !hasAboveTop {
		t.Fatalf("expected upward pan + zoom to push vertices above stage top (SY < -10), proj=%+v", app.Pulse.LastProj)
	}

	upImg := tt.Image()
	topCheckMaxY := int((stageRect.Y - 2) * scale)
	for y := 0; y < topCheckMaxY && y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			got := upImg.RGBAAt(x, y)
			want := baseAbove[y*b.Dx()+x]
			if got.R != want[0] || got.G != want[1] || got.B != want[2] {
				t.Fatalf("脉络 graph overflowed above stage at pixel (%d,%d) [stageTop=%.1f]: got RGBA=%+v want RGBA=%v",
					x, y, stageRect.Y*scale, got, want)
			}
		}
	}

	// 2. Pan strongly downward & zoom in so vertices and edges cross below stage bottom (SY > stageRect.H).
	app.Pulse.PanY = 160
	settleAudit(tt)

	hasBelowBottom := false
	for _, pv := range app.Pulse.LastProj {
		if pv.SY > stageRect.H+10 {
			hasBelowBottom = true
			break
		}
	}
	if !hasBelowBottom {
		t.Fatalf("expected downward pan + zoom to push vertices below stage bottom")
	}

	downImg := tt.Image()
	bottomCheckMinY := int((stageRect.Y + stageRect.H + 6) * scale)
	for y := bottomCheckMinY; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			got := downImg.RGBAAt(x, y)
			want := baseAbove[y*b.Dx()+x]
			if got.R != want[0] || got.G != want[1] || got.B != want[2] {
				t.Fatalf("脉络 graph overflowed below stage at pixel (%d,%d) [stageBottom=%.1f]: got RGBA=%+v want RGBA=%v",
					x, y, (stageRect.Y+stageRect.H)*scale, got, want)
			}
		}
	}
}
