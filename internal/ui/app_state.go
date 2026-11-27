package ui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/ai"
	"github.com/pengstem/neromancer/internal/config"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/media"
	"github.com/pengstem/neromancer/internal/service"
	"github.com/pengstem/neromancer/internal/store"
)

// Space identifies the primary spatial layer the user is exploring.
type Space int

const (
	SpaceDashboard Space = iota // 主页 Bento 智库看板
	SpaceArchive                // 思维宫殿（位于主页上方，从上往下拉出）
)

// ArchiveViewMode controls how ideas are organized inside the Mind Palace.
type ArchiveViewMode int

const (
	ArchiveCards ArchiveViewMode = iota // 卡片（默认）
	ArchiveGraph                        // 脉络（Obsidian 暗色关系网 + AI 归档文件目录）
)

// Deps bundles runtime infrastructure & domain services for the native UI.
type Deps struct {
	Config   config.Config
	Store    *store.Store
	AI       *ai.Client
	Media    *media.Manager
	Capture  *service.CaptureService
	Organize *service.OrganizeService
	Incubate *service.IncubateService
	Share    *service.ShareService
}

// App owns the reactive native UI state across Android/iOS mobile and Desktop preview.
type App struct {
	Deps Deps

	mu       sync.Mutex
	pending  []func()
	MainWin  *mygo.Window
	FloatWin *mygo.Window
	Services ui.Services

	// Navigation & Appearance state
	ActiveSpace          Space
	ArchiveMode          ArchiveViewMode
	ActiveTag            string
	SearchQuery          string
	ArchiveSearchOpen    bool
	ArchiveTagDrawerOpen bool
	AnimOrigin           time.Time
	PortalCueOrigin      time.Time

	// Cached vault state (reloaded cleanly on state mutations)
	VaultGen          int
	Nodes             []domain.IdeaNode
	NodeByID          map[string]domain.IdeaNode
	AllEvidences      []domain.RawEvidence
	EvCounts          map[string]int
	Roots             []string
	Children          map[string][]string
	Tags              []service.TagCount
	DashboardDigest   service.DashboardDigest
	SelectedID        string
	SelectedEvidences []domain.RawEvidence
	SleepInsights     []domain.SleepInsight

	// Interactive widget states kept across frames
	DashboardScroll ui.ScrollState
	ArchiveScroll   ui.ScrollState
	DetailScroll    ui.ScrollState
	MarkdownOpts    markdown.Options
	Globe            GlobeState
	SpaceDeck        SpaceDeckState
	Pulse            PulseViewState
	OrbDial          OrbDialState
	DashFabHandle    ui.Handle
	DashDialHandles  [3]ui.Handle
	DashPortalHandle ui.Handle
	ArchiveAddHandle ui.Handle
	CaptureAnim      CaptureTransitionState

	// Optional AI spark prompt refreshed on pull-to-refresh
	ShowcaseSpark string

	// Mobile & Desktop Sheets / Drawers
	DetailSheetOpen     bool
	CaptureDialOpen     bool
	ArchiveDialOpen     bool
	CaptureModalOpen    bool
	EvidenceModalOpen   bool
	SettingsModalOpen   bool
	SleepPageIdx        int
	ShareModalOpen      bool
	QuickOrbOverlayOpen bool

	// Capture form state
	CaptureModalityIdx int // 0=voice, 1=text, 2=image
	CaptureText        string
	CaptureAudioPath   string
	CaptureImagePath   string
	CaptureSceneLoc    string
	CaptureSceneMood   string
	CaptureSceneMusic  string
	CaptureShowScene   bool
	CaptureRecallHits  []domain.RecallMatch
	AppendEvidenceText string

	// Floating Orb state (desktop window + in-app quick pill)
	FloatExpanded  bool
	FloatQuickText string
	FloatOnToggle  func()

	// Share state
	ShareNodeID      string
	LastExportedPath string

	// Settings state
	SettingsBaseURL      string
	SettingsAPIKey       string
	SettingsModel        string
	SettingsTestStatus   string
	SettingsPromptReason string

	// Feedback & async indicator
	BusyTask     string
	BannerText   string
	BannerIsErr  bool
	BannerShowAI bool

	// Decoded image cache for RawEvidence previews
	imgCache map[string]ui.ImageSource
}

// NewApp constructs the reactive App state using Android Material mobile styling.
func NewApp(deps Deps) (*App, error) {
	mobile.SetLook(mobile.Material)

	a := &App{
		Deps:               deps,
		ActiveSpace:        SpaceDashboard,
		ArchiveMode:        ArchiveCards,
		CaptureModalityIdx: 1,
		NodeByID:           make(map[string]domain.IdeaNode),
		EvCounts:           make(map[string]int),
		Children:           make(map[string][]string),
		imgCache:           make(map[string]ui.ImageSource),
		SettingsBaseURL:    deps.Config.DeepSeekBaseURL,
		SettingsAPIKey:     deps.Config.DeepSeekAPIKey,
		SettingsModel:      deps.Config.DeepSeekModel,
		Pulse:              newPulseViewState(),
		OrbDial:            OrbDialState{HoverIdx: -1},
	}

	if deps.Share != nil {
		deps.Share.SetRasterizer(RenderSharePoster)
	}

	if deps.Store != nil {
		if v, _ := deps.Store.GetSetting("deepseek_base_url"); v != "" {
			a.SettingsBaseURL = v
		}
		if v, _ := deps.Store.GetSetting("deepseek_api_key"); v != "" {
			a.SettingsAPIKey = v
		}
		if v, _ := deps.Store.GetSetting("deepseek_model"); v != "" {
			a.SettingsModel = v
		}
		if deps.AI != nil {
			deps.AI.UpdateCredentials(a.SettingsBaseURL, a.SettingsAPIKey, a.SettingsModel)
		}
		if deps.AI == nil || !deps.AI.Configured() {
			if seen, _ := deps.Store.GetSetting(settingDeepSeekOnboardingSeen); seen != "1" {
				a.SettingsModalOpen = true
				a.SettingsPromptReason = settingsPromptFirstLaunch
			}
		}
	}

	if err := a.ReloadVault(); err != nil {
		return nil, err
	}
	return a, nil
}

// Palette returns the Warm Atelier Paper surface palette.
func (a *App) Palette() SurfacePalette {
	return DefaultPalette()
}

// Phase returns the elapsed shader animation phase in seconds, respecting ReduceMotion
// and scheduling continuous frames only while background work or recording is active.
func (a *App) Phase(c *ui.Context) float32 {
	if c == nil || c.Preferences().ReduceMotion {
		return 0.35
	}
	now := c.Now()
	if a.AnimOrigin.IsZero() || now.Before(a.AnimOrigin) {
		a.AnimOrigin = now
	}
	phase := float32(now.Sub(a.AnimOrigin).Seconds())
	if a.BusyTask != "" || (a.Deps.Media != nil && a.Deps.Media.IsRecording()) {
		c.AnimationFrame()
	}
	return phase
}

// ReloadVault refreshes cached nodes, evidences, snapshots, sleep insights, and roam slices.
func (a *App) ReloadVault() error {
	nodes, counts, roots, children, err := a.Deps.Organize.ListVault()
	if err != nil {
		return err
	}
	byID := make(map[string]domain.IdeaNode, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}

	a.Nodes = nodes
	a.NodeByID = byID
	a.EvCounts = counts
	a.Roots = roots
	a.Children = children
	a.Tags = a.Deps.Organize.CollectTags(nodes)
	if allEvs, err := a.Deps.Organize.ListAllEvidences(); err == nil {
		a.AllEvidences = allEvs
	}
	a.VaultGen++

	if a.SelectedID == "" || func() bool { _, ok := byID[a.SelectedID]; return !ok }() {
		if len(nodes) > 0 {
			a.SelectedID = nodes[0].ID
		} else {
			a.SelectedID = ""
		}
	}
	if a.ShareNodeID == "" || func() bool { _, ok := byID[a.ShareNodeID]; return !ok }() {
		a.ShareNodeID = a.SelectedID
	}

	a.refreshSelectedDetails()

	if insights, err := a.Deps.Store.ListActiveSleepInsights(); err == nil {
		a.SleepInsights = insights
	}
	if len(a.SleepInsights) == 0 {
		a.SleepPageIdx = 0
	} else if a.SleepPageIdx >= len(a.SleepInsights) {
		a.SleepPageIdx = len(a.SleepInsights) - 1
	} else if a.SleepPageIdx < 0 {
		a.SleepPageIdx = 0
	}
	a.DashboardDigest = a.Deps.Organize.BuildDashboardDigest(a.Nodes, a.EvCounts, a.SleepInsights)
	return nil
}

// closeOrbDials resets any expanded bottom-right radial circle menu and drag scrub state.
func (a *App) closeOrbDials() {
	a.CaptureDialOpen = false
	a.ArchiveDialOpen = false
	a.OrbDial.Pressing = false
	a.OrbDial.Scrubbing = false
	a.OrbDial.HoverIdx = -1
}

// returnToDashboard transitions smoothly from the Archive Vault back to the Home Dashboard.
func (a *App) returnToDashboard() {
	a.closeOrbDials()
	a.ActiveSpace = SpaceDashboard
}

// enterArchiveWithTag transitions smoothly from Dashboard into the Mind Palace with an optional tag filter.
func (a *App) enterArchiveWithTag(tag string) {
	a.closeOrbDials()
	a.ActiveTag = cleanTagLabel(tag)
	a.SearchQuery = ""
	a.ArchiveSearchOpen = false
	a.ArchiveTagDrawerOpen = false
	a.ActiveSpace = SpaceArchive
	a.ArchiveMode = ArchiveCards
	a.ArchiveScroll.Y = 0
}

// filterByTagFromDetail jumps from a tag chip inside Detail directly into Mind Palace filtered by that tag.
func (a *App) filterByTagFromDetail(tag string) {
	a.DetailSheetOpen = false
	a.enterArchiveWithTag(tag)
}

// enterArchiveWithTerm transitions directly from a tapped Globe word into the Mind Palace filtered by tag or search query.
func (a *App) enterArchiveWithTerm(term service.WordCloudTerm) {
	a.closeOrbDials()
	a.ActiveSpace = SpaceArchive
	a.ArchiveMode = ArchiveCards
	a.ArchiveScroll.Y = 0
	a.ArchiveSearchOpen = false
	a.ArchiveTagDrawerOpen = false
	if tag := cleanTagLabel(term.FilterTag); tag != "" {
		a.ActiveTag = tag
		a.SearchQuery = ""
	} else {
		a.ActiveTag = ""
		a.SearchQuery = strings.TrimSpace(term.Word)
	}
	filtered := a.filteredArchiveNodes()
	if len(filtered) > 0 {
		targetID := filtered[0].ID
		if term.TopNodeID != "" {
			for _, n := range filtered {
				if n.ID == term.TopNodeID {
					targetID = n.ID
					break
				}
			}
		}
		a.SelectNode(targetID)
	}
}

// TriggerGlobeCaptureRipple arms a gentle fly-in ripple and rotation toward a newly captured idea's primary keyword.
func (a *App) TriggerGlobeCaptureRipple(node domain.IdeaNode) {
	term := ""
	for _, t := range node.Tags {
		if tr := strings.TrimSpace(t); tr != "" {
			term = tr
			break
		}
	}
	if term == "" {
		term = trimText(node.Title, 8)
	}
	a.Globe.RipplePending = true
	a.Globe.RippleTerm = term
}

// openNodeDetail opens the selected node in Detail Sheet (phone) or Archive Master-Detail split (desktop).
func (a *App) openNodeDetail(c *ui.Context, id string) {
	a.closeOrbDials()
	a.SelectNode(id)
	phone := true
	if c != nil {
		phone = a.isPhoneLayout(c)
	}
	if phone {
		a.DetailSheetOpen = true
	} else {
		a.ActiveSpace = SpaceArchive
	}
}

func (a *App) refreshSelectedDetails() {
	if a.SelectedID == "" {
		a.SelectedEvidences = nil
		return
	}
	if evs, err := a.Deps.Store.ListEvidencesByNode(a.SelectedID); err == nil {
		a.SelectedEvidences = evs
	}
}

// SelectNode switches the active detail node, reveals its AI folder in Pulse view, and reloads its raw evidences.
func (a *App) SelectNode(id string) {
	a.SelectedID = id
	if n, ok := a.NodeByID[id]; ok && a.Pulse.CollapsedFolders != nil {
		delete(a.Pulse.CollapsedFolders, domain.ResolveIdeaFolder(n))
	}
	a.refreshSelectedDetails()
}

// SelectedNode returns the currently focused IdeaNode if present.
func (a *App) SelectedNode() (domain.IdeaNode, bool) {
	n, ok := a.NodeByID[a.SelectedID]
	return n, ok
}

// Dispatch safely schedules UI state mutations on the UI thread and requests a redraw.
func (a *App) Dispatch(fn func()) {
	if a.MainWin != nil {
		a.MainWin.Update(fn)
	} else {
		a.mu.Lock()
		a.pending = append(a.pending, fn)
		svc := a.Services
		a.mu.Unlock()
		svc.Invalidate()
	}
	if a.FloatWin != nil {
		a.FloatWin.Invalidate()
	}
}

// FlushPending executes any queued UI state mutations on the UI thread (used in headless Tester).
func (a *App) FlushPending() {
	a.mu.Lock()
	if len(a.pending) == 0 {
		a.mu.Unlock()
		return
	}
	batch := a.pending
	a.pending = nil
	a.mu.Unlock()
	for _, fn := range batch {
		fn()
	}
}

// RunAsync executes a DeepSeek or IO background task while updating BusyTask cleanly.
func (a *App) RunAsync(label string, work func(ctx context.Context) error) {
	a.BusyTask = label
	a.BannerText = ""
	a.BannerShowAI = false
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 70*time.Second)
		defer cancel()
		err := work(ctx)
		a.Dispatch(func() {
			a.BusyTask = ""
			if err != nil {
				a.handleAsyncError(err)
			}
			_ = a.ReloadVault()
		})
	}()
}

func (a *App) markSettingsOnboardingSeen() {
	if a.Deps.Store != nil {
		_ = a.Deps.Store.SetSetting(settingDeepSeekOnboardingSeen, "1")
	}
}

func (a *App) openSettingsPrompt(reason, statusMsg string) {
	a.closeOrbDials()
	a.SettingsPromptReason = reason
	a.SettingsTestStatus = statusMsg
	a.SettingsModalOpen = true
}

func (a *App) openSettingsForIssue(err error) {
	a.BannerText = ""
	a.BannerIsErr = false
	a.BannerShowAI = false
	status := ""
	if err != nil {
		if errors.Is(err, ai.ErrMissingAPIKey) {
			status = err.Error()
		} else {
			status = "连接失败：" + err.Error()
		}
	}
	a.openSettingsPrompt(settingsPromptUnavailable, status)
}

func (a *App) handleAsyncError(err error) {
	if err == nil {
		return
	}
	if ai.IsConfigUnusableError(err) {
		if !a.SettingsModalOpen {
			a.openSettingsForIssue(err)
		}
		return
	}
	if !a.SettingsModalOpen {
		a.setErrorBanner(err.Error())
	}
}

// CheckAIHealthAsync silently verifies configured DeepSeek credentials at startup and
// automatically opens the DeepSeek settings sheet if the config is detected to be unusable.
func (a *App) CheckAIHealthAsync() {
	if a.Deps.AI == nil || !a.Deps.AI.Configured() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_, err := a.Deps.AI.TestConnection(ctx)
		if ai.IsConfigUnusableError(err) {
			a.Dispatch(func() {
				if !a.SettingsModalOpen {
					a.openSettingsForIssue(err)
				}
			})
		}
	}()
}

func (a *App) setBanner(msg string) {
	a.BannerText = msg
	a.BannerIsErr = false
	a.BannerShowAI = false
}

func (a *App) setErrorBanner(msg string) {
	a.BannerText = msg
	a.BannerIsErr = true
	a.BannerShowAI = strings.Contains(msg, "DeepSeek")
}

func (a *App) isPhoneLayout(c *ui.Context) bool {
	w, _ := c.Size()
	return w < 760
}

// RootView builds the Android-first responsive UI frame-by-frame without a bottom navigation bar.
func (a *App) RootView(c *ui.Context) {
	a.FlushPending()
	a.mu.Lock()
	a.Services = c.Services()
	a.mu.Unlock()

	pal := a.Palette()

	ui.Themed(c, NeromancerTheme()).Fill().Children(func() {
		bgHidden := a.CaptureModalOpen && a.CaptureAnim.Phase == CaptureAnimOpen
		ui.Column(c).Fill().Background(pal.Canvas).AccessHidden(a.CaptureModalOpen).Children(func() {
			if !bgHidden {
				if a.QuickOrbOverlayOpen {
					a.renderInAppQuickOrbBar(c, pal)
				}

				a.renderSpaceDeck(c, pal)
			}
		})

		// Non-shifting floating capsule toast overlay
		a.renderBanner(c, pal)

		// Mobile-friendly bottom sheets & full-screen capture morph layer
		ui.Box(c).Key("modal_slot_detail").Children(func() { a.renderDetailSheet(c) })
		a.renderCaptureModal(c)
		ui.Box(c).Key("modal_slot_evidence").Children(func() { a.renderEvidenceModal(c) })
		ui.Box(c).Key("modal_slot_settings").Children(func() { a.renderSettingsModal(c) })
		ui.Box(c).Key("modal_slot_share").Children(func() { a.renderShareModal(c) })
	})
}

func (a *App) renderBanner(c *ui.Context, pal SurfacePalette) {
	if a.CaptureAnim.Phase == CaptureAnimClosingPalace && !a.BannerIsErr {
		return
	}
	topSafe, _, bottomSafe, _ := c.SafeArea()
	phone := a.isPhoneLayout(c)
	posOverlay := func(el ui.Element) ui.Element {
		el.Absolute().Left(16).Right(16).ZIndex(95).AlignItems(ui.Center).PassThrough()
		if phone {
			return el.Top(52 + topSafe)
		}
		return el.Bottom(48 + bottomSafe)
	}

	if a.BusyTask != "" && a.BannerText == "" {
		posOverlay(ui.Column(c).Key("banner_busy_overlay")).
			Children(func() {
				ui.Row(c).Padding(7, 14).Gap(8).AlignItems(ui.Center).MaxWidth(420).
					Radius(RadiusPill).Background(pal.Card).Border(1, pal.Border).
					Shadow(0, 6, 18, 0, ui.RGBA(18, 22, 33, 0.08)).
					Children(func() {
						ui.Spinner(c).Size(14, 14)
						ui.Text(c, a.BusyTask).FontSize(FontMeta).FontWeight(500).TextColor(pal.Ink).SingleLine()
					})
			})
		return
	}
	if a.BannerText == "" {
		return
	}

	th := c.Theme()
	dotCol := th.Success
	border := pal.Border
	fg := pal.Ink
	if a.BannerIsErr {
		dotCol = th.Danger
		border = th.Danger.Alpha(0.35)
		fg = th.Danger
	}
	posOverlay(ui.Column(c).Key("banner_toast_overlay")).
		Children(func() {
			ui.Row(c).Padding(6, 8, 6, 13).Gap(8).AlignItems(ui.Center).MaxWidth(440).
				Radius(RadiusPill).Background(pal.Card).Border(1, border).
				Shadow(0, 6, 18, 0, ui.RGBA(18, 22, 33, 0.09)).
				Children(func() {
					ui.Box(c).Size(7, 7).Radius(4).Background(dotCol).Shrink(0)
					ui.Text(c, a.BannerText).FontSize(FontMeta).FontWeight(500).TextColor(fg).SingleLine()
					if a.BannerShowAI {
						if PillAction(c, "banner_ai_cfg", IconSliders, "去设置", true).Clicked() {
							a.SettingsModalOpen = true
						}
					}
					if GhostIconAction(c, "banner_close", IconClose, "关闭提示", false).Size(24, 24).Clicked() {
						a.BannerText = ""
						a.BannerShowAI = false
					}
				})
		})
}

func (a *App) renderInAppQuickOrbBar(c *ui.Context, pal SurfacePalette) {
	StyleSurfaceCard(
		ui.Row(c).FillWidth().Padding(8, 12).Margin(10, 16, 0, 16).Gap(8).AlignItems(ui.Center),
		pal,
	).Children(func() {
		ui.Icon(c, IconBulb).Size(16, 16).TextColor(ColorVermilion).Shrink(0)
		ui.TextInput(c, &a.FloatQuickText).Placeholder("随手记一句…").Background(pal.SoftPanel).Grow(1)
		if PillAction(c, "quick_orb_submit", IconPlus, "记下来", true).Clicked() && strings.TrimSpace(a.FloatQuickText) != "" {
			a.submitFloatingCapture("")
			a.QuickOrbOverlayOpen = false
		}
	})
}
