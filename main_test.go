package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/config"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/service"
	"github.com/pengstem/neromancer/internal/store"
	appui "github.com/pengstem/neromancer/internal/ui"
)

func newTestApp(t *testing.T) (*appui.App, func()) {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Config{
		DeepSeekBaseURL: "https://api.deepseek.com",
		DeepSeekModel:   "deepseek-flash",
		DataDir:         tmp,
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SeedIfEmpty(); err != nil {
		t.Fatal(err)
	}
	mm := media.NewManager(cfg.AudioDir(), cfg.ImageDir())
	aiClient := ai.NewClient(cfg.DeepSeekBaseURL, "", cfg.DeepSeekModel)

	app, err := appui.NewApp(appui.Deps{
		Config:   cfg,
		Store:    st,
		AI:       aiClient,
		Media:    mm,
		Capture:  service.NewCaptureService(st, aiClient, mm),
		Organize: service.NewOrganizeService(st),
		Incubate: service.NewIncubateService(st, aiClient),
		Share:    service.NewShareService(st, cfg.ExportDir()),
	})
	if err != nil {
		t.Fatal(err)
	}
	return app, func() { _ = st.Close() }
}

func settleSheet(tt *ui.Tester) {
	tt.Frame()
	for i := 0; i < 24 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
	}
	tt.Frame()
	tt.Frame()
}

func assertNoEmojis(t *testing.T, viewName string, texts []string) {
	t.Helper()
	const explicit = "✨🪶🧭🗂📤💡💬🚀🎞🖼📲💊🌊⚡🔴⏹📂🔌💾⚙🫧🌙📍🔗📁🕸🃏"
	for _, txt := range texts {
		for _, r := range txt {
			if strings.ContainsRune(explicit, r) || (r >= 0x1F300 && r <= 0x1FAFF) {
				t.Fatalf("anti-slop violation in %s: found forbidden emoji %q in text %q", viewName, r, txt)
			}
		}
	}
}

func TestEndToEndAndroidMobileWorkflow(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// Test at Android phone portrait dimensions (412x892 DIPs)
	tt := ui.NewTester(app.RootView, 412, 892)

	// 1. Homepage Dashboard renders Option B Bento sections & has NO bottom navigation bar.
	for _, want := range []string{"AI 灵感画像", "本周积累", "AI 提炼关键词矩阵", "串联与待深化追问", "语音速记", "图文落稿"} {
		if !tt.HasText(want) {
			t.Fatalf("expected Bento Dashboard text %q, texts=%v", want, tt.Texts())
		}
	}
	if _, ok := tt.Find("进入灵感档案室"); !ok {
		t.Fatalf("expected bottom portal labeled \"进入灵感档案室\", texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("灵感"); ok {
		t.Fatalf("expected bottom nav tab \"灵感\" to be removed")
	}
	if _, ok := tt.Find("整理"); ok {
		t.Fatalf("expected bottom nav tab \"整理\" to be removed")
	}
	assertNoEmojis(t, "dashboard", tt.Texts())

	// 2. Tap "记一笔" -> open Capture bottom sheet, type a thought, and save locally ("记下来").
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("click quick capture header CTA: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("记一条灵感") || !tt.HasText("记下来") {
		t.Fatalf("expected capture sheet to open, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "capture_modal", tt.Texts())

	tt.Type("在阳台装一个自动虹吸滴灌棉线")
	if err := tt.Click("记下来"); err != nil {
		t.Fatalf("click 记下来: %v", err)
	}
	settleSheet(tt)

	// Newly captured note must auto-match botanical icon on landing.
	var capturedNode *domain.IdeaNode
	for i := range app.Nodes {
		if strings.Contains(app.Nodes[i].Title, "阳台") {
			capturedNode = &app.Nodes[i]
			break
		}
	}
	if capturedNode == nil {
		t.Fatalf("expected newly captured node in vault")
	}
	if capturedNode.Visual.MotifType != domain.IconKeyBotanical {
		t.Fatalf("expected auto-matched botanical icon for balcony thought, got %q", capturedNode.Visual.MotifType)
	}

	// 3. Enter Inspiration Archive Vault via "进入灵感档案室".
	if err := tt.Click("进入灵感档案室"); err != nil {
		t.Fatalf("enter archive vault: %v", err)
	}
	settleSheet(tt)
	if app.ActiveSpace != appui.SpaceArchive || app.ArchiveMode != appui.ArchiveCards {
		t.Fatalf("expected SpaceArchive/ArchiveCards, got space=%v mode=%v", app.ActiveSpace, app.ArchiveMode)
	}
	if !tt.HasText("灵感档案室") || !tt.HasText("用一个蓝色花瓶养一株郁金香") || !tt.HasText("在阳台装一个自动虹吸滴灌棉线") {
		t.Fatalf("expected archive vault cards, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "archive_cards", tt.Texts())

	// Switch Archive mode: Cards -> Outline -> Cards.
	if err := tt.Click("目录"); err != nil {
		t.Fatalf("switch archive mode to outline: %v", err)
	}
	settleSheet(tt)
	if app.ArchiveMode != appui.ArchiveOutline {
		t.Fatalf("expected ArchiveOutline, got %v", app.ArchiveMode)
	}
	assertNoEmojis(t, "archive_outline", tt.Texts())

	if err := tt.Click("档案卡"); err != nil {
		t.Fatalf("switch archive mode back to cards: %v", err)
	}
	settleSheet(tt)

	// 4. Tap Tulip card in Archive -> Detail Sheet opens with close button, "AI 帮忙" & "原始记录".
	if err := tt.Click("用一个蓝色花瓶养一株郁金香"); err != nil {
		t.Fatalf("click tulip card in archive: %v", err)
	}
	settleSheet(tt)
	if !app.DetailSheetOpen || app.SelectedID != "orb_tulip" {
		t.Fatalf("expected tulip DetailSheetOpen, open=%v id=%s", app.DetailSheetOpen, app.SelectedID)
	}
	if !tt.HasText("AI 帮忙") || !tt.HasText("原始记录") {
		t.Fatalf("expected detail actions, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("关闭"); !ok {
		t.Fatalf("expected explicit close button inside detail sheet")
	}
	assertNoEmojis(t, "detail_sheet", tt.Texts())

	if err := tt.Click("原始记录"); err != nil {
		t.Fatalf("click raw evidence button: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("买一盆郁金香放在阳台上养") || !tt.HasText("郁金香的花瓶应该是蓝色的") {
		t.Fatalf("expected both original tulip raw evidences inside evidence sheet, texts=%v", tt.Texts())
	}
	if !tt.HasText("播放原声") {
		t.Fatalf("raw evidence sheet missing audio playback control, texts=%v", tt.Texts())
	}
	if r, ok := tt.Find("补充"); !ok || r.Y+r.H > 892 {
		t.Fatalf("raw evidence append composer \"补充\" clipped/off-screen: ok=%v rect=%+v", ok, r)
	}
	assertNoEmojis(t, "raw_evidence_sheet", tt.Texts())

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close evidence sheet: %v", err)
	}
	settleSheet(tt)
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close detail sheet: %v", err)
	}
	settleSheet(tt)
	if app.DetailSheetOpen {
		t.Fatalf("expected DetailSheetOpen=false after clicking close")
	}

	// 5. Switch to Roam & Poster Studio inside Archive, verify slices & export high-DPI PNG poster.
	if err := tt.Click("漫游"); err != nil {
		t.Fatalf("switch to roam tab in archive: %v", err)
	}
	settleSheet(tt)
	if app.ArchiveMode != appui.ArchiveRoam {
		t.Fatalf("expected ArchiveRoam, got %v", app.ArchiveMode)
	}
	if !tt.HasText("把游戏引擎 FrameGraph 渲染管线类比成厨房出菜台") {
		t.Fatalf("expected seeded roam slice in roam view")
	}
	if !tt.HasText("选一条灵感") || !tt.HasText("碰撞") {
		t.Fatalf("expected roam controls, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "archive_roam", tt.Texts())

	if err := tt.Click("导出海报"); err != nil {
		t.Fatalf("click export poster: %v", err)
	}
	if app.LastExportedPath == "" {
		t.Fatalf("expected exported PNG path to be set")
	}
	f, err := os.Open(app.LastExportedPath)
	if err != nil {
		t.Fatalf("open exported PNG poster: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode exported PNG poster: %v", err)
	}
	if img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 1640 {
		t.Fatalf("expected 2x 600x820 poster (1200x1640), got %v", img.Bounds())
	}
	if filepath.Ext(app.LastExportedPath) != ".png" {
		t.Fatalf("unexpected extension: %s", app.LastExportedPath)
	}

	// 6. Return cleanly back up to Dashboard via "回到主页看板".
	if err := tt.Click("回到主页看板"); err != nil {
		t.Fatalf("click back to dashboard: %v", err)
	}
	settleSheet(tt)
	if app.ActiveSpace != appui.SpaceDashboard {
		t.Fatalf("expected SpaceDashboard after returning, got %v", app.ActiveSpace)
	}
}

func TestFloatingOrbWindowWidget(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	ft := ui.NewTester(app.FloatingOrbView, 96, 96)
	ft.ClickAt(48, 48)
	if !app.FloatExpanded {
		t.Fatalf("expected clicking floating companion to expand quick capture")
	}
	ft.SetSize(360, 250)
	ft.Frame()
	if !ft.HasText("随手记") || !ft.HasText("记下来") {
		t.Fatalf("expected expanded floating widget UI, texts=%v", ft.Texts())
	}
	assertNoEmojis(t, "floating_orb_expanded", ft.Texts())
}
