package ui

import (
	"math"
	"testing"
	"time"
)

func TestSpaceDeckPullDownOpensArchiveAndShowsBothFloors(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	// Top pull-down portal must live at the top of the Dashboard.
	portalRect, ok := tt.Find("进入思维宫殿")
	if !ok {
		t.Fatalf("expected top Mind Palace portal on Dashboard")
	}
	if portalRect.Y > 80 {
		t.Fatalf("expected top Mind Palace portal near top edge (Y <= 80), got %+v", portalRect)
	}

	// Finger drags top-to-bottom from Dashboard header/hero area to pull open Mind Palace from above.
	startX := float32(180)
	startY := portalRect.Y + portalRect.H*0.5
	tt.FingerDown(startX, startY)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(startX, startY+60)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(startX, startY+180)
	tt.Advance(20 * time.Millisecond)
	tt.Frame()

	// Mid-pull: WeChat "second floor" connected transition — both floors visibly move together.
	if !tt.HasText("思维宫殿") || !tt.HasText("AI 灵感画像") {
		t.Fatalf("expected both Mind Palace and Dashboard floors mounted mid-pull, texts=%v", tt.Texts())
	}
	if !tt.HasText("松手打开思维宫殿") {
		t.Fatalf("expected release cue \"松手打开思维宫殿\" after crossing pull threshold, texts=%v", tt.Texts())
	}
	if tt.HasText("松手回到主页看板") {
		t.Fatalf("unexpected Archive return release cue while pulling down from Dashboard, texts=%v", tt.Texts())
	}
	maybeSaveAuditPNG(t, resolveVisualAuditDir(), tt, "ui_space_deck_mid_pull.png")

	tt.FingerUp(startX, startY+180)
	settleAudit(tt)

	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("expected SpaceArchive after downward pull past threshold, got %v", app.ActiveSpace)
	}
	backRect, ok := tt.Find("回到主页看板")
	if !ok {
		t.Fatalf("expected bottom return handle \"回到主页看板\" in Archive")
	}
	if backRect.Y < 800 {
		t.Fatalf("expected return handle at bottom of Archive (Y >= 800), got %+v", backRect)
	}
	if tt.HasText("AI 灵感画像") {
		t.Fatalf("expected Dashboard floor unmounted once settled on Archive")
	}
}

func TestSpaceDeckShortPullBouncesBackToDashboard(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	portalRect, ok := tt.Find("进入思维宫殿")
	if !ok {
		t.Fatalf("missing top portal")
	}
	sx := float32(180)
	sy := portalRect.Y + portalRect.H*0.5

	tt.FingerDown(sx, sy)
	tt.Advance(30 * time.Millisecond)
	tt.FingerMove(sx, sy+30)
	tt.Advance(120 * time.Millisecond)
	tt.FingerMove(sx, sy+48)
	tt.Advance(140 * time.Millisecond) // pause so release velocity is ~0
	tt.FingerUp(sx, sy+48)
	settleAudit(tt)

	if app.ActiveSpace != SpaceDashboard {
		t.Fatalf("expected short pull below threshold to bounce back to SpaceDashboard, got %v", app.ActiveSpace)
	}
	if math.Abs(float64(app.SpaceDeck.Offset.Y+892)) > 2 {
		t.Fatalf("expected SpaceDeck.Offset.Y settled at -892, got %v", app.SpaceDeck.Offset.Y)
	}
}

func TestSpaceDeckPushUpFromArchiveBottomHandleReturnsHome(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
	app.ActiveSpace = SpaceArchive
	settleAudit(tt)

	backRect, ok := tt.Find("回到主页看板")
	if !ok {
		t.Fatalf("missing bottom return handle in Archive")
	}
	sx := backRect.X + backRect.W*0.5
	sy := backRect.Y + backRect.H*0.5

	tt.FingerDown(sx, sy)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(sx, sy-80)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(sx, sy-200)
	tt.Advance(20 * time.Millisecond)
	tt.Frame()

	if !tt.HasText("AI 灵感画像") || !tt.HasText("思维宫殿") {
		t.Fatalf("expected both floors visible while pushing Mind Palace upward, texts=%v", tt.Texts())
	}
	if !tt.HasText("松手回到主页看板") {
		t.Fatalf("expected release cue \"松手回到主页看板\", texts=%v", tt.Texts())
	}
	if tt.HasText("松手打开思维宫殿") {
		t.Fatalf("unexpected Dashboard open release cue while pushing up from Archive, texts=%v", tt.Texts())
	}

	tt.FingerUp(sx, sy-200)
	settleAudit(tt)

	if app.ActiveSpace != SpaceDashboard {
		t.Fatalf("expected SpaceDashboard after pushing Archive handle upward, got %v", app.ActiveSpace)
	}
}

func TestSpaceDeckSmallScrollableScreenBoundaryHandoff(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	// 360x620 forces DashboardScroll and ArchiveScroll to have MaxY > 0.
	tt := newFastTester(app.RootView, 360, 620)
	settleAudit(tt)

	if app.DashboardScroll.MaxY <= 0 {
		t.Fatalf("expected scrollable Dashboard on 360x620, got MaxY=%v", app.DashboardScroll.MaxY)
	}

	// 1. Finger drags downward inside scrollable Hero area while DashboardScroll.Y == 0 -> opens Archive.
	heroRect, ok := tt.Find("AI 灵感画像")
	if !ok {
		t.Fatalf("missing hero card on 360x620")
	}
	hx := heroRect.X + heroRect.W*0.5
	hy := heroRect.Y + heroRect.H*0.5

	tt.FingerDown(hx, hy)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(hx, hy+50)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(hx, hy+130)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(hx, hy+210)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(hx, hy+210)
	settleAudit(tt)

	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("expected top-boundary downward pull on scrollable Dashboard to open SpaceArchive, got %v (offset=%v)", app.ActiveSpace, app.SpaceDeck.Offset.Y)
	}

	// 2. Dragging bottom handle upward returns to Dashboard even when ArchiveScroll.Y == 0.
	backRect, ok := tt.Find("回到主页看板")
	if !ok {
		t.Fatalf("missing bottom return handle on 360x620 Archive")
	}
	bx := backRect.X + backRect.W*0.5
	by := backRect.Y + backRect.H*0.5

	tt.FingerDown(bx, by)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(bx, by-90)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(bx, by-200)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(bx, by-200)
	settleAudit(tt)

	if app.ActiveSpace != SpaceDashboard {
		t.Fatalf("expected bottom handle push-up on 360x620 to return to SpaceDashboard, got %v", app.ActiveSpace)
	}

	// 3. Scroll Archive Cards to bottom on 360x620, then drag finger upward inside list -> hands off back to Dashboard.
	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards
	settleAudit(tt)
	if app.ArchiveScroll.MaxY > 0 {
		app.ArchiveScroll.Y = app.ArchiveScroll.MaxY
		tt.Frame()
		tt.FingerDown(180, 420)
		tt.Advance(20 * time.Millisecond)
		tt.FingerMove(180, 360)
		tt.Advance(20 * time.Millisecond)
		tt.FingerMove(180, 270)
		tt.Advance(20 * time.Millisecond)
		tt.FingerMove(180, 170)
		tt.Advance(20 * time.Millisecond)
		tt.FingerUp(180, 170)
		settleAudit(tt)
		if app.ActiveSpace != SpaceDashboard {
			t.Fatalf("expected bottom-boundary upward push on scrollable Archive to return to SpaceDashboard, got %v (offset=%v)", app.ActiveSpace, app.SpaceDeck.Offset.Y)
		}
	}
}

func TestSpaceDeckGlobeVerticalDragDoesNotOpenArchive(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	terms := app.DashboardDigest.WordCloud
	if len(terms) == 0 {
		t.Fatalf("expected non-empty WordCloud terms")
	}
	r, ok := tt.Find(terms[0].Word)
	if !ok {
		t.Fatalf("could not locate globe term %q", terms[0].Word)
	}
	cx := r.X + r.W*0.5
	cy := r.Y + r.H*0.5
	rotBefore := app.Globe.Rot

	// Drag straight DOWN inside the 3D Globe: must rotate Globe, NOT pull down Archive.
	tt.FingerDown(cx, cy)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(cx, cy+90)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(cx, cy+190)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(cx, cy+190)
	tt.Frame()

	if app.ActiveSpace != SpaceDashboard {
		t.Fatalf("expected downward drag inside 3D Globe to keep SpaceDashboard, got %v", app.ActiveSpace)
	}
	diff := 0.0
	for i := 0; i < 9; i++ {
		diff += math.Abs(app.Globe.Rot[i] - rotBefore[i])
	}
	if diff < 0.05 {
		t.Fatalf("expected 3D Globe to rotate on downward drag, diff=%v", diff)
	}
}
