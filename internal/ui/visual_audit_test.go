package ui

import (
	"image"
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
)

func resolveVisualAuditDir(t *testing.T) string {
	t.Helper()
	if dir := strings.TrimSpace(os.Getenv("NEROMANCER_VISUAL_AUDIT_DIR")); dir != "" {
		return dir
	}
	return t.TempDir()
}

func newAuditTestApp(t *testing.T) (*App, func()) {
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

	app, err := NewApp(Deps{
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

func settleAudit(tt *ui.Tester) {
	tt.Frame()
	for i := 0; i < 6 && tt.FramePending(); i++ {
		tt.Advance(100 * time.Millisecond)
	}
	tt.Frame()
}

func assertAuditNoEmojis(t *testing.T, viewName string, texts []string) {
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

func saveAuditPNG(t *testing.T, auditDir string, img *image.RGBA, filename string) {
	t.Helper()
	if img == nil || img.Bounds().Empty() {
		t.Fatalf("rendered image for %s is nil or empty", filename)
	}
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", auditDir, err)
	}
	outPath := filepath.Join(auditDir, filename)
	f, err := os.Create(outPath)
	if err != nil {
		t.Fatalf("create %s: %v", outPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode %s: %v", outPath, err)
	}
}

func TestVisualAuditAndAntiSlopScreenshots(t *testing.T) {
	auditDir := resolveVisualAuditDir(t)
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := ui.NewTester(app.RootView, 412, 892)
	settleAudit(tt)

	// 1. Showcase Day + Orb Field + Frosted Dock
	for _, want := range []string{"星海", "思维空间", "漫游分享", "记灵感"} {
		if !tt.HasText(want) {
			t.Fatalf("showcase missing dock label %q", want)
		}
	}
	if _, ok := tt.Find("语音速记"); !ok {
		t.Fatalf("showcase missing dock voice button labeled \"语音速记\"")
	}
	assertAuditNoEmojis(t, "showcase_day", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_showcase_day.png")

	// 2. Showcase Night Observatory via DarkMode toggle
	if err := tt.Click("夜间模式"); err != nil {
		t.Fatalf("click night mode: %v", err)
	}
	settleAudit(tt)
	if !app.DarkMode {
		t.Fatalf("expected DarkMode=true after toggle")
	}
	assertAuditNoEmojis(t, "showcase_night", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_showcase_night.png")

	// Restore Day Mode for daylight atelier screenshots.
	if err := tt.Click("夜间模式"); err != nil {
		t.Fatalf("restore day mode: %v", err)
	}
	settleAudit(tt)

	// 3. Voice-First Capture Sheet open via dock "语音速记"
	if err := tt.Click("语音速记"); err != nil {
		t.Fatalf("click voice capture dock button: %v", err)
	}
	settleAudit(tt)
	if !app.CaptureModalOpen || app.CaptureModalityIdx != 0 {
		t.Fatalf("expected CaptureModalOpen=true and CaptureModalityIdx=0, got open=%v idx=%d", app.CaptureModalOpen, app.CaptureModalityIdx)
	}
	if !tt.HasText("记下此刻") || !tt.HasText("收入星海") || !tt.HasText("轻触说话") {
		t.Fatalf("capture voice sheet missing expected labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "capture_voice", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_capture_voice.png")

	// Also verify text & image capture tabs have zero emojis before closing.
	if err := tt.Click("文字"); err != nil {
		t.Fatalf("switch capture tab to text: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "capture_text", tt.Texts())
	if err := tt.Click("画面"); err != nil {
		t.Fatalf("switch capture tab to image: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "capture_image", tt.Texts())
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("close capture sheet: %v", err)
	}
	settleAudit(tt)

	// 4. Mind Space Outline view
	if err := tt.Click("思维空间"); err != nil {
		t.Fatalf("switch to mind space: %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceMind || app.MindOrgMode != OrgOutline {
		t.Fatalf("expected SpaceMind/OrgOutline, got space=%v mode=%v", app.ActiveSpace, app.MindOrgMode)
	}
	assertAuditNoEmojis(t, "mind_outline", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_mind_outline.png")

	// 5. Mind Space Constellation Graph view via "切换组织视图"
	if err := tt.Click("切换组织视图"); err != nil {
		t.Fatalf("cycle mind org mode to graph: %v", err)
	}
	settleAudit(tt)
	if app.MindOrgMode != OrgGraph {
		t.Fatalf("expected OrgGraph, got %v", app.MindOrgMode)
	}
	assertAuditNoEmojis(t, "mind_constellation", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_mind_constellation.png")

	// Cycle to Scene Cards to verify zero emojis there as well, then back to Outline.
	if err := tt.Click("切换组织视图"); err != nil {
		t.Fatalf("cycle mind org mode to cards: %v", err)
	}
	settleAudit(tt)
	if app.MindOrgMode != OrgCards {
		t.Fatalf("expected OrgCards, got %v", app.MindOrgMode)
	}
	assertAuditNoEmojis(t, "mind_cards", tt.Texts())
	if err := tt.Click("切换组织视图"); err != nil {
		t.Fatalf("cycle mind org mode back to outline: %v", err)
	}
	settleAudit(tt)

	// 6. Tulip Detail Sheet with stage track & spoken quote
	if err := tt.Click("用一个蓝色花瓶养一株郁金香"); err != nil {
		t.Fatalf("click tulip node in mind outline: %v", err)
	}
	settleAudit(tt)
	if !app.DetailSheetOpen || app.SelectedID != "orb_tulip" {
		t.Fatalf("expected tulip detail sheet open, open=%v id=%s", app.DetailSheetOpen, app.SelectedID)
	}
	if !tt.HasText("原始证据") || !tt.HasText("AI 帮忙") {
		t.Fatalf("detail sheet missing expected action labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "detail_sheet", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_detail_sheet.png")

	// 7. Raw Evidence tape with audio waveform bars & immutable tulip quotes
	if err := tt.Click("原始证据"); err != nil {
		t.Fatalf("click raw evidence button: %v", err)
	}
	settleAudit(tt)
	if !app.EvidenceModalOpen {
		t.Fatalf("expected EvidenceModalOpen=true")
	}
	if !tt.HasText("买一盆郁金香放在阳台上养") || !tt.HasText("郁金香的花瓶应该是蓝色的") {
		t.Fatalf("raw evidence sheet missing immutable tulip quotes, texts=%v", tt.Texts())
	}
	if !tt.HasText("播放原声") || !tt.HasText("关闭") {
		t.Fatalf("raw evidence sheet missing audio/close controls, texts=%v", tt.Texts())
	}
	if r, ok := tt.Find("追加"); !ok || r.Y+r.H > 892 {
		t.Fatalf("raw evidence append composer \"追加\" clipped/off-screen: ok=%v rect=%+v", ok, r)
	}
	assertAuditNoEmojis(t, "raw_evidence", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_raw_evidence.png")

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close raw evidence sheet: %v", err)
	}
	settleAudit(tt)
	app.DetailSheetOpen = false
	settleAudit(tt)

	// Verify Sleep & Settings modals also contain zero emojis.
	if err := tt.Click("睡眠孵化"); err != nil {
		t.Fatalf("open sleep modal: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "sleep_modal", tt.Texts())
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close sleep modal: %v", err)
	}
	settleAudit(tt)

	if err := tt.Click("DeepSeek 设置"); err != nil {
		t.Fatalf("open settings modal: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "settings_modal", tt.Texts())
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close settings modal: %v", err)
	}
	settleAudit(tt)

	// 8. Roam & Poster Studio view
	if err := tt.Click("漫游分享"); err != nil {
		t.Fatalf("switch to roam view: %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceRoam {
		t.Fatalf("expected SpaceRoam, got %v", app.ActiveSpace)
	}
	if !tt.HasText("选择记忆球") {
		t.Fatalf("expected \"选择记忆球\" selector label, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("分享海报"); !ok {
		t.Fatalf("expected dock action \"分享海报\" in roam view, texts=%v", tt.Texts())
	}
	if !tt.HasText("导出海报") || !tt.HasText("碰撞") {
		t.Fatalf("roam view missing expected labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "roam_poster", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_roam_poster.png")

	// Verify dock button "分享海报" opens share modal without emojis
	if err := tt.Click("分享海报"); err != nil {
		t.Fatalf("click dock action \"分享海报\": %v", err)
	}
	settleAudit(tt)
	if !app.ShareModalOpen {
		t.Fatalf("expected ShareModalOpen=true after clicking 分享海报")
	}
	assertAuditNoEmojis(t, "share_modal", tt.Texts())
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close share modal: %v", err)
	}
	settleAudit(tt)

	// 9. Export 1200x1640 PNG poster and copy to ui_exported_poster.png
	if err := tt.Click("导出海报"); err != nil {
		t.Fatalf("click export poster: %v", err)
	}
	if app.LastExportedPath == "" {
		t.Fatalf("expected LastExportedPath to be non-empty")
	}
	rawPoster, err := os.ReadFile(app.LastExportedPath)
	if err != nil {
		t.Fatalf("read exported poster %s: %v", app.LastExportedPath, err)
	}
	f, err := os.Open(app.LastExportedPath)
	if err != nil {
		t.Fatalf("open exported poster: %v", err)
	}
	defer f.Close()
	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode exported poster: %v", err)
	}
	if decoded.Bounds().Dx() != 1200 || decoded.Bounds().Dy() != 1640 {
		t.Fatalf("expected 1200x1640 exported poster, got %v", decoded.Bounds())
	}
	if err := os.WriteFile(filepath.Join(auditDir, "ui_exported_poster.png"), rawPoster, 0o644); err != nil {
		t.Fatalf("copy ui_exported_poster.png: %v", err)
	}
}
