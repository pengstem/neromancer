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
		tt.Advance(100 * time.Millisecond)
	}
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

	// 1. Showcase renders seeded Memory Orbs & zero-emoji Frosted Dock ("灵感", "整理", "漫游", "记灵感", "语音速记").
	if !tt.HasText("用一个蓝色花瓶养一株郁金香") {
		t.Fatalf("expected seeded tulip orb in showcase, texts=%v", tt.Texts())
	}
	for _, want := range []string{"灵感", "整理", "漫游", "记灵感"} {
		if !tt.HasText(want) {
			t.Fatalf("expected dock label %q in showcase, texts=%v", want, tt.Texts())
		}
	}
	if _, ok := tt.Find("语音速记"); !ok {
		t.Fatalf("expected voice quick capture dock button labeled \"语音速记\"")
	}
	if _, ok := tt.Find("夜间模式"); ok {
		t.Fatalf("expected dark mode toggle \"夜间模式\" to be removed")
	}
	assertNoEmojis(t, "showcase", tt.Texts())

	// 2. Tap "记灵感" -> open Capture bottom sheet, type a thought, and save locally ("记下来").
	if err := tt.Click("记灵感"); err != nil {
		t.Fatalf("click quick capture FAB: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("记一条灵感") {
		t.Fatalf("expected capture sheet \"记一条灵感\" to open, texts=%v", tt.Texts())
	}
	if !tt.HasText("记下来") {
		t.Fatalf("expected primary capture CTA \"记下来\", texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "capture_modal", tt.Texts())

	tt.Type("在阳台装一个自动虹吸滴灌棉线")
	if err := tt.Click("记下来"); err != nil {
		t.Fatalf("click 记下来: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("在阳台装一个自动虹吸滴灌棉线") {
		t.Fatalf("expected newly captured orb to appear in showcase")
	}

	// 3. Switch to Mind Space via BottomBar -> FAB morphs to "切换视图".
	if err := tt.Click("整理"); err != nil {
		t.Fatalf("tap Mind Space bottom tab: %v", err)
	}
	settleSheet(tt)
	if app.ActiveSpace != appui.SpaceMind {
		t.Fatalf("expected SpaceMind, got %v", app.ActiveSpace)
	}
	if _, ok := tt.Find("切换视图"); !ok {
		t.Fatalf("expected morphed organize button labeled \"切换视图\" in Mind Space, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "mind_outline", tt.Texts())

	// Cycle organization mode via morphed FAB: Outline -> Graph -> Cards -> Outline.
	if err := tt.Click("切换视图"); err != nil {
		t.Fatalf("cycle org mode 1: %v", err)
	}
	settleSheet(tt)
	if app.MindOrgMode != appui.OrgGraph {
		t.Fatalf("expected OrgGraph after first click, got %v", app.MindOrgMode)
	}
	assertNoEmojis(t, "mind_graph", tt.Texts())

	if err := tt.Click("切换视图"); err != nil {
		t.Fatalf("cycle org mode 2: %v", err)
	}
	settleSheet(tt)
	if app.MindOrgMode != appui.OrgCards {
		t.Fatalf("expected OrgCards after second click, got %v", app.MindOrgMode)
	}
	assertNoEmojis(t, "mind_cards", tt.Texts())

	if err := tt.Click("切换视图"); err != nil {
		t.Fatalf("cycle org mode 3: %v", err)
	}
	settleSheet(tt)
	if app.MindOrgMode != appui.OrgOutline {
		t.Fatalf("expected OrgOutline after third click, got %v", app.MindOrgMode)
	}

	// 4. Tap Tulip node in Outline -> Detail Sheet slides up -> verify "AI 帮忙" & "原始记录".
	if err := tt.Click("用一个蓝色花瓶养一株郁金香"); err != nil {
		t.Fatalf("click tulip node in mind space: %v", err)
	}
	settleSheet(tt)
	if !app.DetailSheetOpen {
		t.Fatalf("expected DetailSheetOpen on phone width")
	}
	if !tt.HasText("AI 帮忙") {
		t.Fatalf("expected \"AI 帮忙\" button inside detail sheet, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "detail_sheet", tt.Texts())

	if err := tt.Click("原始记录"); err != nil {
		t.Fatalf("click raw evidence button: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("买一盆郁金香放在阳台上养") || !tt.HasText("郁金香的花瓶应该是蓝色的") {
		t.Fatalf("expected both original tulip raw evidences inside evidence sheet, texts=%v", tt.Texts())
	}
	if !tt.HasText("播放原声") || !tt.HasText("关闭") {
		t.Fatalf("raw evidence sheet missing audio/close controls, texts=%v", tt.Texts())
	}
	if r, ok := tt.Find("补充"); !ok || r.Y+r.H > 892 {
		t.Fatalf("raw evidence append composer \"补充\" clipped/off-screen: ok=%v rect=%+v", ok, r)
	}
	assertNoEmojis(t, "raw_evidence_sheet", tt.Texts())

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close evidence sheet: %v", err)
	}
	settleSheet(tt)
	app.DetailSheetOpen = false
	settleSheet(tt)

	// 5. Switch to Roam & Cool Share tab, verify read-only roaming slices & export high-DPI PNG card.
	if err := tt.Click("漫游"); err != nil {
		t.Fatalf("switch to roam & share tab: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("把游戏引擎 FrameGraph 渲染管线类比成厨房出菜台") {
		t.Fatalf("expected seeded roam slice in roam view")
	}
	if !tt.HasText("选一条灵感") {
		t.Fatalf("expected \"选一条灵感\" label in roam poster studio, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("分享海报"); !ok {
		t.Fatalf("expected dock button \"分享海报\" in roam space, texts=%v", tt.Texts())
	}
	if err := tt.Click("分享海报"); err != nil {
		t.Fatalf("click dock button \"分享海报\": %v", err)
	}
	settleSheet(tt)
	if !app.ShareModalOpen {
		t.Fatalf("expected app.ShareModalOpen to be true")
	}
	assertNoEmojis(t, "share_modal", tt.Texts())
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close share modal: %v", err)
	}
	settleSheet(tt)
	if !tt.HasText("碰撞") {
		t.Fatalf("expected \"碰撞\" pill button in roam view, texts=%v", tt.Texts())
	}
	assertNoEmojis(t, "roam_share", tt.Texts())
	for _, txt := range tt.Texts() {
		if strings.Contains(txt, "发表评论") || strings.Contains(txt, "写评论") {
			t.Fatalf("found forbidden forum/comment UI text: %q", txt)
		}
	}

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
}

func TestFloatingOrbWindowWidget(t *testing.T) {
	app, cleanup := newTestApp(t)
	defer cleanup()

	ft := ui.NewTester(app.FloatingOrbView, 96, 96)
	ft.ClickAt(48, 48)
	if !app.FloatExpanded {
		t.Fatalf("expected clicking floating orb to expand quick capture")
	}
	ft.SetSize(360, 250)
	ft.Frame()
	if !ft.HasText("随手记") || !ft.HasText("记下来") {
		t.Fatalf("expected expanded floating orb UI, texts=%v", ft.Texts())
	}
	assertNoEmojis(t, "floating_orb_expanded", ft.Texts())
}
