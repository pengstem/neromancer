package ui

import (
	"fmt"
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
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/service"
	"github.com/pengstem/neromancer/internal/store"
)

func resolveVisualAuditDir() string {
	return strings.TrimSpace(os.Getenv("NEROMANCER_VISUAL_AUDIT_DIR"))
}

func newFastTester(view func(c *ui.Context), width, height int) *ui.Tester {
	return ui.NewTesterWith(view, width, height, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: true},
	})
}

func newAuditTestApp(t testing.TB) (*App, func()) {
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
	for i := 0; i < 8 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
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

func maybeSaveAuditPNG(t *testing.T, auditDir string, tt *ui.Tester, filename string) {
	t.Helper()
	if auditDir == "" {
		return
	}
	saveAuditPNG(t, auditDir, tt.Image(), filename)
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
	auditDir := resolveVisualAuditDir()
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
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
	if r, ok := tt.Find("进入思维宫殿"); !ok || r.Y > 80 {
		t.Fatalf("dashboard missing top pull-down portal \"进入思维宫殿\", got ok=%v rect=%+v", ok, r)
	}
	if _, ok := tt.Find("灵感"); ok {
		t.Fatalf("expected bottom nav bar \"灵感\" to be removed")
	}
	assertAuditNoEmojis(t, "dashboard_bento", tt.Texts())
	maybeSaveAuditPNG(t, auditDir, tt, "ui_dashboard_bento.png")

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
	maybeSaveAuditPNG(t, auditDir, tt, "ui_dashboard_speed_dial.png")

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
	maybeSaveAuditPNG(t, auditDir, tt, "ui_capture_voice.png")

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
		maybeSaveAuditPNG(t, auditDir, tt, "ui_archive_globe_filtered.png")

		if err := tt.Click("回到主页看板"); err != nil {
			t.Fatalf("back to dashboard: %v", err)
		}
		settleAudit(tt)
	}

	// 4. Mind Palace (Cards mode via top portal)
	if err := tt.Click("进入思维宫殿"); err != nil {
		t.Fatalf("enter Mind Palace: %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive || app.ArchiveMode != ArchiveCards {
		t.Fatalf("expected SpaceArchive/ArchiveCards, got space=%v mode=%v", app.ActiveSpace, app.ArchiveMode)
	}
	if !tt.HasText("思维宫殿") || !tt.HasText("用一个蓝色花瓶养一株郁金香") {
		t.Fatalf("mind palace cards missing expected labels, texts=%v", tt.Texts())
	}
	if _, ok := tt.Find("随手记一笔"); !ok {
		t.Fatalf("expected quick-add capture trigger \"随手记一笔\" in Mind Palace header")
	}
	if tt.HasText("记灵感") {
		t.Fatalf("expected duplicate archive capture FAB \"记灵感\" to be removed")
	}
	for _, forbidden := range []string{"闪念", "萌芽", "孵化中", "已沉淀", "变化记录"} {
		if tt.HasText(forbidden) {
			t.Fatalf("expected idea status/stage label %q to be removed from mind palace cards, texts=%v", forbidden, tt.Texts())
		}
	}
	assertAuditNoEmojis(t, "archive_cards", tt.Texts())
	assertAuditNoHashPrefixes(t, "archive_cards", tt.Texts())
	maybeSaveAuditPNG(t, auditDir, tt, "ui_archive_cards.png")

	// 5. Mind Palace Apple Light Graph & AI File Explorer ("脉络") via bottom-right morphed view orb — and verify "漫游" & old header tabs are gone!
	if tt.HasText("漫游") {
		t.Fatalf("expected '漫游' tab to be removed, texts=%v", tt.Texts())
	}
	if tt.HasText("脉络") {
		t.Fatalf("expected top-right header '脉络' tab to be removed before opening bottom-right orb dial, texts=%v", tt.Texts())
	}
	if err := tt.Click("切换视窗（当前卡片）"); err != nil {
		t.Fatalf("open bottom-right archive view orb dial: %v", err)
	}
	settleAudit(tt)
	if !app.ArchiveDialOpen {
		t.Fatalf("expected ArchiveDialOpen=true after tapping view orb")
	}
	if err := tt.Click("脉络"); err != nil {
		t.Fatalf("switch mind palace mode to graph via circular orb: %v", err)
	}
	settleAudit(tt)
	if app.ArchiveMode != ArchiveGraph || app.ArchiveDialOpen {
		t.Fatalf("expected ArchiveGraph & dial closed, got mode=%v open=%v", app.ArchiveMode, app.ArchiveDialOpen)
	}
	if _, ok := tt.Find("切换视窗（当前脉络）"); !ok {
		t.Fatalf("expected bottom-right circle icon/state to switch to 脉络")
	}
	if !tt.HasText("AI 归档目录") || !tt.HasText("器物与光影美学") {
		t.Fatalf("expected AI File Explorer folders in 脉络 view, texts=%v", tt.Texts())
	}
	// Verify Graph stage renders in crisp Apple daylight white (never black!)
	if img := tt.Image(); img != nil {
		px := img.RGBAAt(40, 90)
		if px.R < 230 || px.G < 230 || px.B < 230 {
			t.Fatalf("expected Apple light surface in 脉络 graph stage, got dark pixel RGBA=%+v", px)
		}
	}
	assertAuditNoEmojis(t, "archive_graph", tt.Texts())
	maybeSaveAuditPNG(t, auditDir, tt, "ui_archive_graph.png")

	if err := tt.Click("切换视窗（当前脉络）"); err != nil {
		t.Fatalf("re-open bottom-right archive view orb dial: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("卡片"); err != nil {
		t.Fatalf("switch mind palace mode back to cards: %v", err)
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
	maybeSaveAuditPNG(t, auditDir, tt, "ui_detail_sheet.png")

	// 7. Raw Evidence tape with audio waveform bars & immutable tulip quotes (single-layer sheet, no DetailSheet stacking)
	if err := tt.Click("原始记录"); err != nil {
		t.Fatalf("click raw evidence button: %v", err)
	}
	settleAudit(tt)
	if !app.EvidenceModalOpen {
		t.Fatalf("expected EvidenceModalOpen=true")
	}
	if tt.HasText("灵感详情") {
		t.Fatalf("expected underlying DetailSheet to defer while Raw Evidence sheet is open (no sheet-on-sheet stacking)")
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
	maybeSaveAuditPNG(t, auditDir, tt, "ui_raw_evidence.png")

	if err := tt.Click("关闭"); err != nil {
		t.Fatalf("close raw evidence sheet: %v", err)
	}
	settleAudit(tt)

	// 8. Pure Share Feature inside Detail Sheet (single-layer sheet, no DetailSheet stacking)
	if err := tt.Click("分享"); err != nil {
		t.Fatalf("click share button in detail sheet: %v", err)
	}
	settleAudit(tt)
	if !app.ShareModalOpen {
		t.Fatalf("expected ShareModalOpen=true after clicking 分享")
	}
	if tt.HasText("灵感详情") {
		t.Fatalf("expected underlying DetailSheet to defer while Share sheet is open (no sheet-on-sheet stacking)")
	}
	if !tt.HasText("分享灵感") || !tt.HasText("保存海报") || !tt.HasText("系统分享") {
		t.Fatalf("share sheet missing expected labels, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "share_modal", tt.Texts())
	maybeSaveAuditPNG(t, auditDir, tt, "ui_share_modal.png")

	// 9. Export 1200x1640 PNG poster when saving visual audit artifacts
	if auditDir != "" {
		if err := tt.Click("保存海报"); err != nil {
			t.Fatalf("click save poster: %v", err)
		}
		if app.LastExportedPath == "" {
			t.Fatalf("expected LastExportedPath to be non-empty")
		}
		if strings.Contains(app.BannerText, filepath.Dir(app.LastExportedPath)) {
			t.Fatalf("expected human-friendly export toast without raw directory path, got %q", app.BannerText)
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
	} else {
		if err := tt.Click("关闭"); err != nil {
			t.Fatalf("close share sheet: %v", err)
		}
	}
	settleAudit(tt)

	if app.DetailSheetOpen {
		if err := tt.Click("关闭"); err != nil {
			t.Fatalf("close detail sheet: %v", err)
		}
		settleAudit(tt)
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

	// 11. Responsive layout checks: minimum 360x620 & desktop 920x720
	ttMin := newFastTester(app.RootView, 360, 620)
	app.ActiveSpace = SpaceDashboard
	settleAudit(ttMin)
	if r, ok := ttMin.Find("记一笔"); !ok || r.X+r.W > 360 || r.Y+r.H > 620 {
		t.Fatalf("360px bottom-right capture seal clipped: ok=%v rect=%+v", ok, r)
	}
	maybeSaveAuditPNG(t, auditDir, ttMin, "ui_360x620_dashboard.png")

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards
	settleAudit(ttMin)
	if r, ok := ttMin.Find("切换视窗（当前卡片）"); !ok || r.X+r.W > 360 || r.Y+r.H > 620 {
		t.Fatalf("360px archive bottom-right view orb clipped: ok=%v rect=%+v", ok, r)
	}
	maybeSaveAuditPNG(t, auditDir, ttMin, "ui_360x620_archive.png")

	ttWide := newFastTester(app.RootView, 920, 720)
	app.ActiveSpace = SpaceDashboard
	settleAudit(ttWide)
	maybeSaveAuditPNG(t, auditDir, ttWide, "ui_920x720_dashboard.png")

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards
	app.SelectNode("orb_tulip")
	settleAudit(ttWide)
	maybeSaveAuditPNG(t, auditDir, ttWide, "ui_920x720_archive.png")
}

func TestDashboardCaptureSpeedDialInteractions(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := newFastTester(app.RootView, 412, 892)
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
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("close image capture sheet: %v", err)
	}
	settleAudit(tt)

	// 5. Continuous press-hold-drag on "记一笔": hold still pops out 3 circles, gliding finger switches hovered circle, releasing opens it!
	fabRect, ok := tt.Find("记一笔")
	if !ok {
		t.Fatalf("missing 记一笔 circular FAB")
	}
	cx := fabRect.X + fabRect.W*0.5
	cy := fabRect.Y + fabRect.H*0.5

	tt.FingerDown(cx, cy)
	tt.Advance(170 * time.Millisecond) // hold still past 150ms threshold -> pops out 3 circles!
	tt.Frame()
	if !app.CaptureDialOpen {
		t.Fatalf("expected CaptureDialOpen=true after holding down on 记一笔")
	}
	// Verify each popped-out option is a standalone 50x50 square/circle (W == H)
	for _, lbl := range []string{"语音", "文字", "图文"} {
		optR, found := tt.Find(lbl)
		if !found {
			t.Fatalf("missing popped-out circular option %q", lbl)
		}
		if optR.W < 44 || optR.H < 44 || optR.W-optR.H > 2 || optR.H-optR.W > 2 {
			t.Fatalf("expected %q to be a standalone circle (W≈H), got rect=%+v", lbl, optR)
		}
	}

	// Keep holding and glide finger across 语音 -> 图文 -> back to 文字, then release!
	voiceR, _ := tt.Find("语音")
	imgR, _ := tt.Find("图文")
	txtR, _ := tt.Find("文字")

	tt.FingerMove(voiceR.X+voiceR.W*0.5, voiceR.Y+voiceR.H*0.5)
	tt.Frame()
	if app.OrbDial.HoverIdx != 2 {
		t.Fatalf("expected HoverIdx=2 (语音) when holding over 语音 circle, got %d", app.OrbDial.HoverIdx)
	}

	tt.FingerMove(imgR.X+imgR.W*0.5, imgR.Y+imgR.H*0.5)
	tt.Frame()
	if app.OrbDial.HoverIdx != 0 {
		t.Fatalf("expected HoverIdx=0 (图文) when holding over 图文 circle, got %d", app.OrbDial.HoverIdx)
	}

	tt.FingerMove(txtR.X+txtR.W*0.5, txtR.Y+txtR.H*0.5)
	tt.Frame()
	if app.OrbDial.HoverIdx != 1 {
		t.Fatalf("expected HoverIdx=1 (文字) when gliding back over 文字 circle, got %d", app.OrbDial.HoverIdx)
	}

	tt.FingerUp(txtR.X+txtR.W*0.5, txtR.Y+txtR.H*0.5)
	settleAudit(tt)
	if app.CaptureDialOpen || !app.CaptureModalOpen || app.CaptureModalityIdx != 1 {
		t.Fatalf("expected releasing held finger on 文字 circle to open text capture, got dial=%v modal=%v idx=%d",
			app.CaptureDialOpen, app.CaptureModalOpen, app.CaptureModalityIdx)
	}
	if err := tt.Click("收起"); err != nil {
		t.Fatalf("close text capture modal: %v", err)
	}
	settleAudit(tt)

	// 6. Transition to Mind Palace & verify identical press-drag feel on the morphed view switcher circle!
	if err := tt.Click("进入思维宫殿"); err != nil {
		t.Fatalf("enter Mind Palace: %v", err)
	}
	settleAudit(tt)

	archOrbRect, ok := tt.Find("切换视窗（当前卡片）")
	if !ok {
		t.Fatalf("missing morphed bottom-right view switcher circle in Mind Palace")
	}
	ax := archOrbRect.X + archOrbRect.W*0.5
	ay := archOrbRect.Y + archOrbRect.H*0.5

	// Press and drag upward directly onto "脉络" circle and release to switch from default 卡片 to 脉络!
	tt.FingerDown(ax, ay)
	tt.Advance(30 * time.Millisecond)
	tt.FingerMove(ax, ay-35) // drag > 8px immediately pops out [脉络, 卡片] circles!
	tt.Frame()
	if !app.ArchiveDialOpen {
		t.Fatalf("expected ArchiveDialOpen=true on upward drag from view switcher circle")
	}
	graphOrbR, ok := tt.Find("脉络")
	if !ok {
		t.Fatalf("missing popped-out 脉络 circle")
	}
	tt.FingerMove(graphOrbR.X+graphOrbR.W*0.5, graphOrbR.Y+graphOrbR.H*0.5)
	tt.Frame()
	if app.OrbDial.HoverIdx != 0 {
		t.Fatalf("expected HoverIdx=0 (脉络) while dragging over 脉络 circle, got %d", app.OrbDial.HoverIdx)
	}
	tt.FingerUp(graphOrbR.X+graphOrbR.W*0.5, graphOrbR.Y+graphOrbR.H*0.5)
	settleAudit(tt)

	if app.ArchiveMode != ArchiveGraph || app.ArchiveDialOpen {
		t.Fatalf("expected ArchiveMode=ArchiveGraph after press-drag release on 脉络 circle, got mode=%v open=%v",
			app.ArchiveMode, app.ArchiveDialOpen)
	}
	if _, ok := tt.Find("切换视窗（当前脉络）"); !ok {
		t.Fatalf("expected main circle icon/state to update to 切换视窗（当前脉络）")
	}
}

func assertAuditNoHashPrefixes(t *testing.T, viewName string, texts []string) {
	t.Helper()
	for _, txt := range texts {
		trimmed := strings.TrimSpace(txt)
		if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "＃") {
			t.Fatalf("found ugly '#' prefix in %s text %q", viewName, txt)
		}
	}
}

func TestMindPalaceTagConvergenceAndZeroHashSlop(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	// Populate 18 extra ideas with diverse auto-generated tags (including legacy '#' dirty tags)
	// to verify the filter bar never overflows horizontally and '#' never leaks into UI.
	extraTags := []string{
		"生活器物", "声音采集", "空间叙事", "合成器", "胶片摄影",
		"建筑光影", "手冲咖啡", "编译原理", "发酵实验", "独立游戏",
		"#旧版脏井号", "夜间散步", "植物标本", "交互手感", "纸张肌理",
	}
	now := time.Now()
	for i := 0; i < 18; i++ {
		t1 := extraTags[i%len(extraTags)]
		t2 := extraTags[(i+3)%len(extraTags)]
		tags := []string{t1, t2}
		if i == 17 {
			tags = []string{"苔藓微景观"}
		}
		err := app.Deps.Store.UpsertNode(domain.IdeaNode{
			ID:                    fmt.Sprintf("orb_extra_%d", i),
			Title:                 fmt.Sprintf("测试灵感第 %d 号", i+1),
			ConversationalSummary: "验证多标签场景下思维宫殿的收敛筛选与去井号贴片。",
			Tags:                  tags,
			Theme:                 domain.ThemeSky,
			Weight:                0.7,
			CreatedAt:             now.Add(time.Duration(-i-10) * time.Hour),
			UpdatedAt:             now.Add(time.Duration(-i-10) * time.Hour),
		})
		if err != nil {
			t.Fatalf("UpsertNode %d: %v", i, err)
		}
	}
	if err := app.ReloadVault(); err != nil {
		t.Fatalf("ReloadVault: %v", err)
	}

	app.ActiveSpace = SpaceArchive
	app.ArchiveMode = ArchiveCards

	// 1. Verify narrow 360px phone screen keeps '+N' overflow trigger strictly inside viewport.
	ttNarrow := newFastTester(app.RootView, 360, 620)
	settleAudit(ttNarrow)
	moreRect, ok := ttNarrow.Find("展开全部主题")
	if !ok || moreRect.X+moreRect.W > 360 {
		t.Fatalf("expected '+N' overflow filter button within 360px width, ok=%v rect=%+v", ok, moreRect)
	}
	assertAuditNoHashPrefixes(t, "mind_palace_360", ttNarrow.Texts())

	// 2. Verify standard 412px phone screen opens multi-row tag drawer, selects an overflow tag, and surfaces it inline.
	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)
	assertAuditNoHashPrefixes(t, "mind_palace_412", tt.Texts())
	if r, ok := tt.Find("苔藓微景观"); ok && r.Y < 400 {
		t.Fatalf("expected long-tail tag '苔藓微景观' not in top inline bar before opening '+N' drawer, got rect=%+v", r)
	}

	if err := tt.Click("展开全部主题"); err != nil {
		t.Fatalf("click expand all topics: %v", err)
	}
	settleAudit(tt)
	if !app.ArchiveTagDrawerOpen {
		t.Fatalf("expected ArchiveTagDrawerOpen=true")
	}
	if !tt.HasText("旧版脏井号") || !tt.HasText("苔藓微景观") {
		t.Fatalf("expected cleaned & long-tail tags inside drawer, texts=%v", tt.Texts())
	}
	assertAuditNoHashPrefixes(t, "mind_palace_drawer", tt.Texts())

	if err := tt.Click("苔藓微景观"); err != nil {
		t.Fatalf("click overflow tag '苔藓微景观' in drawer: %v", err)
	}
	settleAudit(tt)
	if app.ArchiveTagDrawerOpen {
		t.Fatalf("expected ArchiveTagDrawerOpen=false after selecting tag")
	}
	if app.ActiveTag != "苔藓微景观" {
		t.Fatalf("expected ActiveTag='苔藓微景观', got %q", app.ActiveTag)
	}
	if r, ok := tt.Find("苔藓微景观"); !ok || r.Y > 140 || !tt.HasText("测试灵感第 18 号") {
		t.Fatalf("expected selected overflow tag pinned in top inline bar (Y <= 140) & matching card visible, ok=%v rect=%+v", ok, r)
	}

	// 3. Verify Detail Sheet & Evidence Sheet & Roam have zero '#' prefixes.
	app.ActiveTag = ""
	app.SelectNode("orb_tulip")
	app.DetailSheetOpen = true
	settleAudit(tt)
	assertAuditNoHashPrefixes(t, "detail_sheet_no_hash", tt.Texts())

	if err := tt.Click("原始记录"); err != nil {
		t.Fatalf("open raw evidence sheet: %v", err)
	}
	settleAudit(tt)
	assertAuditNoHashPrefixes(t, "raw_evidence_no_hash", tt.Texts())
	app.EvidenceModalOpen = false
	app.DetailSheetOpen = false

	app.ShareNodeID = "orb_tulip"
	app.ShareModalOpen = true
	settleAudit(tt)
	assertAuditNoHashPrefixes(t, "share_poster_no_hash", tt.Texts())
}
