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
	for i := 0; i < 6 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
	}
	tt.Frame()
}

func TestEndToEndAndroidMobileWorkflow(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	// Test at Android phone portrait dimensions (412x892 DIPs) with ReduceMotion
	// so interactive E2E assertions settle immediately without rendering 60fps transition frames.
	tt := ui.NewTesterWith(app.RootView, 412, 892, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true},
	})

	// 1. Capture a new text thought via bottom-right Speed-Dial and verify persistence + auto-matched icon.
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("click bottom-right capture seal: %v", err)
	}
	settleSheet(tt)
	if err := tt.Click("文字"); err != nil {
		t.Fatalf("click text dial pill: %v", err)
	}
	settleSheet(tt)
	if app.CaptureDialOpen || !app.CaptureModalOpen {
		t.Fatalf("expected dial closed and capture sheet open, dial=%v modal=%v", app.CaptureDialOpen, app.CaptureModalOpen)
	}

	tt.Type("在阳台装一个自动虹吸滴灌棉线")
	if err := tt.Click("记下来"); err != nil {
		t.Fatalf("click 记下来: %v", err)
	}
	settleSheet(tt)

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

	// 2. Enter Mind Palace and confirm newly captured card is visible alongside seeded nodes.
	if err := tt.Click("进入思维宫殿"); err != nil {
		t.Fatalf("enter Mind Palace: %v", err)
	}
	settleSheet(tt)
	if app.ActiveSpace != appui.SpaceArchive || app.ArchiveMode != appui.ArchiveCards {
		t.Fatalf("expected SpaceArchive/ArchiveCards, got space=%v mode=%v", app.ActiveSpace, app.ArchiveMode)
	}
	if !tt.HasText("用一个蓝色花瓶养一株郁金香") || !tt.HasText("在阳台装一个自动虹吸滴灌棉线") {
		t.Fatalf("expected newly captured & seeded cards in archive, texts=%v", tt.Texts())
	}

	// 3. Switch to Obsidian Graph & AI File Explorer ("脉络") and verify auto-clustering + pure Share poster export.
	if err := tt.Click("脉络"); err != nil {
		t.Fatalf("switch to graph tab in archive: %v", err)
	}
	settleSheet(tt)
	if app.ArchiveMode != appui.ArchiveGraph {
		t.Fatalf("expected ArchiveGraph, got %v", app.ArchiveMode)
	}
	if !tt.HasText("器物与光影美学") || !tt.HasText("在阳台装一个自动虹吸滴灌棉线") {
		t.Fatalf("expected newly captured note grouped into AI folder in 脉络 view, texts=%v", tt.Texts())
	}

	if err := tt.Click("在阳台装一个自动虹吸滴灌棉线"); err != nil {
		t.Fatalf("click captured note row in AI file explorer: %v", err)
	}
	settleSheet(tt)
	if !app.DetailSheetOpen {
		t.Fatalf("expected DetailSheetOpen=true after clicking file row")
	}
	if err := tt.Click("分享"); err != nil {
		t.Fatalf("click 分享 in detail sheet: %v", err)
	}
	settleSheet(tt)

	if err := tt.Click("保存海报"); err != nil {
		t.Fatalf("click 保存海报: %v", err)
	}
	settleSheet(tt)
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

	if app.DetailSheetOpen {
		if err := tt.Click("关闭"); err != nil {
			t.Fatalf("close detail sheet: %v", err)
		}
		settleSheet(tt)
	}

	// 4. Return cleanly back to Dashboard via "回到主页看板".
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

	ft := ui.NewTesterWith(app.FloatingOrbView, 96, 96, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true},
	})
	ft.ClickAt(48, 48)
	if !app.FloatExpanded {
		t.Fatalf("expected clicking floating companion to expand quick capture")
	}
	ft.SetSize(360, 250)
	ft.Frame()
	if !ft.HasText("随手记") || !ft.HasText("记下来") {
		t.Fatalf("expected expanded floating widget UI, texts=%v", ft.Texts())
	}
}
