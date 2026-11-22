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
	for i := 0; i < 24 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
	}
	tt.Frame()
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

	// 1. Homepage Bento Dashboard + 3D Inspiration Globe + unified bottom-right Vermilion Seal (no title/icon, no search, no duplicates, no bottom nav bar)
	for _, want := range []string{"AI 灵感画像", "本周积累", "灵感星云球", "串联与待深化追问"} {
		if !tt.HasText(want) {
			t.Fatalf("dashboard missing expected label %q", want)
		}
	}
	if tt.HasText("Neromancer") {
		t.Fatalf("expected homepage app title \"Neromancer\" to be removed, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("搜索灵感"); ok {
		t.Fatalf("expected homepage search button \"搜索灵感\" to be removed, texts=%v", tt.Texts())
	}
	if tt.HasText("语音速记") || tt.HasText("图文落稿") {
		t.Fatalf("expected duplicate dashboard capture tiles to be removed, texts=%v", tt.Texts())
	}
	if r, ok := tt.Find("记一笔"); !ok || r.X < 300 || r.Y < 750 {
		t.Fatalf("expected unified bottom-right \"记一笔\" seal button, got ok=%v rect=%+v", ok, r)
	}
	if r, ok := tt.Find("进入灵感档案室"); !ok || r.Y > 80 {
		t.Fatalf("dashboard missing top pull-down portal \"进入灵感档案室\", got ok=%v rect=%+v", ok, r)
	}
	if _, ok := tt.Find("灵感"); ok {
		t.Fatalf("expected bottom nav bar \"灵感\" to be removed")
	}
	assertAuditNoEmojis(t, "dashboard_bento", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_dashboard_bento.png")

	// 2. Vermilion Seal Speed-Dial -> Voice-First Capture Sheet
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("click bottom-right capture seal: %v", err)
	}
	settleAudit(tt)
	if !app.CaptureDialOpen {
		t.Fatalf("expected CaptureDialOpen=true after clicking bottom-right seal")
	}
	if !tt.HasText("语音") || !tt.HasText("文字") || !tt.HasText("图文") {
		t.Fatalf("speed-dial missing modality pills, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "dashboard_speed_dial", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_dashboard_speed_dial.png")

	if err := tt.Click("语音"); err != nil {
		t.Fatalf("click voice dial pill: %v", err)
	}
	settleAudit(tt)
	if app.CaptureDialOpen {
		t.Fatalf("expected CaptureDialOpen=false after selecting voice modality")
	}
	if !app.CaptureModalOpen || app.CaptureModalityIdx != 0 {
		t.Fatalf("expected CaptureModalOpen=true and CaptureModalityIdx=0, got open=%v idx=%d", app.CaptureModalOpen, app.CaptureModalityIdx)
	}
	if !tt.HasText("记一条灵感") || !tt.HasText("记下来") || !tt.HasText("点一下开始说话") {
		t.Fatalf("capture voice sheet missing expected labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "capture_voice", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_capture_voice.png")

	if err := tt.Click("文字"); err != nil {
		t.Fatalf("switch capture tab to text: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "capture_text", tt.Texts())
	if err := tt.Click("图片"); err != nil {
		t.Fatalf("switch capture tab to image: %v", err)
	}
	settleAudit(tt)
	assertAuditNoEmojis(t, "capture_image", tt.Texts())
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("close capture sheet: %v", err)
	}
	settleAudit(tt)

	// 3. Tapping a foreground Globe word directly enters SpaceArchive with automatic filter (no homepage focus popup)
	if len(app.DashboardDigest.WordCloud) > 0 {
		firstTerm := app.DashboardDigest.WordCloud[0]
		if err := tt.Click(firstTerm.Word); err != nil {
			t.Fatalf("click globe foreground term %q: %v", firstTerm.Word, err)
		}
		settleAudit(tt)
		if tt.HasText("看相关档案") {
			t.Fatalf("expected no homepage focus preview card after clicking %q, texts=%v", firstTerm.Word, tt.Texts())
		}
		if app.ActiveSpace != SpaceArchive || (firstTerm.FilterTag != "" && app.ActiveTag != firstTerm.FilterTag) || (firstTerm.FilterTag == "" && app.SearchQuery != firstTerm.Word) {
			t.Fatalf("expected SpaceArchive filtered by %q, got space=%v tag=%q query=%q", firstTerm.Word, app.ActiveSpace, app.ActiveTag, app.SearchQuery)
		}
		assertAuditNoEmojis(t, "archive_globe_filtered", tt.Texts())
		saveAuditPNG(t, auditDir, tt.Image(), "ui_archive_globe_filtered.png")

		if err := tt.Click("回到主页看板"); err != nil {
			t.Fatalf("back to dashboard: %v", err)
		}
		settleAudit(tt)
	}

	// 4. Inspiration Archive Vault (Cards mode via bottom portal)
	if err := tt.Click("进入灵感档案室"); err != nil {
		t.Fatalf("enter archive vault: %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive || app.ArchiveMode != ArchiveCards {
		t.Fatalf("expected SpaceArchive/ArchiveCards, got space=%v mode=%v", app.ActiveSpace, app.ArchiveMode)
	}
	if !tt.HasText("灵感档案室") || !tt.HasText("用一个蓝色花瓶养一株郁金香") {
		t.Fatalf("archive cards missing expected labels, texts=%v", tt.Texts())
	}
	if tt.HasText("记灵感") {
		t.Fatalf("expected duplicate archive capture FAB \"记灵感\" to be removed")
	}
	for _, forbidden := range []string{"闪念", "萌芽", "孵化中", "已沉淀", "变化记录"} {
		if tt.HasText(forbidden) {
			t.Fatalf("expected idea status/stage label %q to be removed from archive cards, texts=%v", forbidden, tt.Texts())
		}
	}
	assertAuditNoEmojis(t, "archive_cards", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_archive_cards.png")

	// 5. Archive Outline mode
	if err := tt.Click("目录"); err != nil {
		t.Fatalf("switch archive mode to outline: %v", err)
	}
	settleAudit(tt)
	if app.ArchiveMode != ArchiveOutline {
		t.Fatalf("expected ArchiveOutline, got %v", app.ArchiveMode)
	}
	assertAuditNoEmojis(t, "archive_outline", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_archive_outline.png")

	if err := tt.Click("档案卡"); err != nil {
		t.Fatalf("switch archive mode back to cards: %v", err)
	}
	settleAudit(tt)

	// 6. Tulip Detail Sheet with close button & spoken quote
	if err := tt.Click("用一个蓝色花瓶养一株郁金香"); err != nil {
		t.Fatalf("click tulip card in archive: %v", err)
	}
	settleAudit(tt)
	if !app.DetailSheetOpen || app.SelectedID != "orb_tulip" {
		t.Fatalf("expected tulip detail sheet open, open=%v id=%s", app.DetailSheetOpen, app.SelectedID)
	}
	if !tt.HasText("原始记录") || !tt.HasText("AI 帮忙") {
		t.Fatalf("detail sheet missing expected action labels, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("关闭"); !ok {
		t.Fatalf("detail sheet missing explicit close button")
	}
	for _, forbidden := range []string{"闪念", "萌芽", "孵化中", "已沉淀", "变化记录"} {
		if tt.HasText(forbidden) {
			t.Fatalf("expected idea status/stage label %q to be removed from detail sheet, texts=%v", forbidden, tt.Texts())
		}
	}
	assertAuditNoEmojis(t, "detail_sheet", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_detail_sheet.png")

	// 7. Raw Evidence tape with audio waveform bars & immutable tulip quotes
	if err := tt.Click("原始记录"); err != nil {
		t.Fatalf("click raw evidence button: %v", err)
	}
	settleAudit(tt)
	if !app.EvidenceModalOpen {
		t.Fatalf("expected EvidenceModalOpen=true")
	}
	if !tt.HasText("买一盆郁金香放在阳台上养") || !tt.HasText("郁金香的花瓶应该是蓝色的") {
		t.Fatalf("raw evidence sheet missing immutable tulip quotes, texts=%v", tt.Texts())
	}
	if !tt.HasText("播放原声") {
		t.Fatalf("raw evidence sheet missing audio control, texts=%v", tt.Texts())
	}
	if r, ok := tt.Find("补充"); !ok || r.Y+r.H > 892 {
		t.Fatalf("raw evidence append composer \"补充\" clipped/off-screen: ok=%v rect=%+v", ok, r)
	}
	assertAuditNoEmojis(t, "raw_evidence", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_raw_evidence.png")

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close raw evidence sheet: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close detail sheet: %v", err)
	}
	settleAudit(tt)

	// 8. Roam & Poster Studio inside Archive
	if err := tt.Click("漫游"); err != nil {
		t.Fatalf("switch archive to roam mode: %v", err)
	}
	settleAudit(tt)
	if app.ArchiveMode != ArchiveRoam {
		t.Fatalf("expected ArchiveRoam, got %v", app.ArchiveMode)
	}
	if !tt.HasText("选一条灵感") || !tt.HasText("导出海报") || !tt.HasText("碰撞") {
		t.Fatalf("roam view missing expected labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "archive_roam", tt.Texts())
	saveAuditPNG(t, auditDir, tt.Image(), "ui_archive_roam.png")

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

	// 10. Verify Sleep & Settings modals open cleanly from Dashboard with zero emojis
	if err := tt.Click("回到主页看板"); err != nil {
		t.Fatalf("back to dashboard: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("灵感串联"); err != nil {
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

	// 11. Responsive layout screenshots: minimum 360x620 & desktop 920x720
	ttMin := ui.NewTester(app.RootView, 360, 620)
	app.ActiveSpace = SpaceDashboard
	settleAudit(ttMin)
	if r, ok := ttMin.Find("记一笔"); !ok || r.X+r.W > 360 || r.Y+r.H > 620 {
		t.Fatalf("360px bottom-right capture seal clipped: ok=%v rect=%+v", ok, r)
	}
	saveAuditPNG(t, auditDir, ttMin.Image(), "ui_360x620_dashboard.png")

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards
	settleAudit(ttMin)
	if r, ok := ttMin.Find("漫游"); !ok || r.X+r.W > 360 {
		t.Fatalf("360px archive header mode switch clipped: ok=%v rect=%+v", ok, r)
	}
	saveAuditPNG(t, auditDir, ttMin.Image(), "ui_360x620_archive.png")

	ttWide := ui.NewTester(app.RootView, 920, 720)
	app.ActiveSpace = SpaceDashboard
	settleAudit(ttWide)
	saveAuditPNG(t, auditDir, ttWide.Image(), "ui_920x720_dashboard.png")

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards
	app.SelectNode("orb_tulip")
	settleAudit(ttWide)
	saveAuditPNG(t, auditDir, ttWide.Image(), "ui_920x720_archive.png")
}

func TestDashboardCaptureSpeedDialInteractions(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := ui.NewTester(app.RootView, 412, 892)
	settleAudit(tt)

	// 1. Open dial then dismiss via backdrop click
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("open speed dial: %v", err)
	}
	settleAudit(tt)
	if !app.CaptureDialOpen {
		t.Fatalf("expected CaptureDialOpen=true")
	}
	if err := tt.Click("收起速记菜单"); err != nil {
		t.Fatalf("click speed dial backdrop: %v", err)
	}
	settleAudit(tt)
	if app.CaptureDialOpen || app.CaptureModalOpen {
		t.Fatalf("expected dial & modal closed after backdrop tap, got dial=%v modal=%v", app.CaptureDialOpen, app.CaptureModalOpen)
	}

	// 2. Open dial then dismiss via seal close button ("收起")
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("re-open speed dial: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("click seal close button: %v", err)
	}
	settleAudit(tt)
	if app.CaptureDialOpen || app.CaptureModalOpen {
		t.Fatalf("expected dial & modal closed after seal toggle, got dial=%v modal=%v", app.CaptureDialOpen, app.CaptureModalOpen)
	}

	// 3. Open dial -> "文字" opens text capture tab (idx=1) and closes dial
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("open speed dial for text: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("文字"); err != nil {
		t.Fatalf("click text pill: %v", err)
	}
	settleAudit(tt)
	if app.CaptureDialOpen || !app.CaptureModalOpen || app.CaptureModalityIdx != 1 {
		t.Fatalf("expected text capture sheet open, got dial=%v modal=%v idx=%d", app.CaptureDialOpen, app.CaptureModalOpen, app.CaptureModalityIdx)
	}
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("close text capture sheet: %v", err)
	}
	settleAudit(tt)

	// 4. Open dial -> "图文" opens image capture tab (idx=2) and closes dial
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("open speed dial for image: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("图文"); err != nil {
		t.Fatalf("click image pill: %v", err)
	}
	settleAudit(tt)
	if app.CaptureDialOpen || !app.CaptureModalOpen || app.CaptureModalityIdx != 2 {
		t.Fatalf("expected image capture sheet open, got dial=%v modal=%v idx=%d", app.CaptureDialOpen, app.CaptureModalOpen, app.CaptureModalityIdx)
	}
}
