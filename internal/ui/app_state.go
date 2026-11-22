package ui

import (
	"context"
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
	ArchiveCards   ArchiveViewMode = iota // 卡片（默认）
	ArchiveOutline                        // 脉络树
	ArchiveRoam                           // 漫游与海报分享
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

	// Cached vault state (reloaded cleanly on state mutations)
	Nodes             []domain.IdeaNode
	NodeByID          map[string]domain.IdeaNode
	EvCounts          map[string]int
	Roots             []string
	Children          map[string][]string
	Tags              []service.TagCount
	DashboardDigest   service.DashboardDigest
	SelectedID        string
	SelectedEvidences []domain.RawEvidence
	RoamSlices        []domain.RoamSlice
	SleepInsights     []domain.SleepInsight

	// Interactive widget states kept across frames
	OutlineState    ui.OutlineState[string]
	OutlineChoice   string
	DashboardScroll ui.ScrollState
	ArchiveScroll   ui.ScrollState
	RoamScroll      ui.ScrollState
	DetailScroll    ui.ScrollState
	MarkdownOpts    markdown.Options
	Globe           GlobeState
	SpaceDeck       SpaceDeckState

	// Optional AI spark prompt refreshed on pull-to-refresh
	ShowcaseSpark string

	// Mobile & Desktop Sheets / Drawers
	DetailSheetOpen     bool
	CaptureDialOpen     bool
	CaptureModalOpen    bool
	EvidenceModalOpen   bool
	SettingsModalOpen   bool
	SleepModalOpen      bool
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
	CaptureAutoOrg     bool
	CaptureRecallHits  []domain.RecallMatch
	AppendEvidenceText string

	// Floating Orb state (desktop window + in-app quick pill)
	FloatExpanded  bool
	FloatQuickText string
	FloatOnToggle  func()

	// Share / Roam state
	ShareNodeID       string
	LastExportedPath  string
	CollisionTargetID string

	// Settings state
	SettingsBaseURL    string
	SettingsAPIKey     string
	SettingsModel      string
	SettingsTestStatus string

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
		CaptureModalityIdx: 1, // Quick text/voice dual-ready
		CaptureAutoOrg:     true,
		NodeByID:           make(map[string]domain.IdeaNode),
		EvCounts:           make(map[string]int),
		Children:           make(map[string][]string),
		imgCache:           make(map[string]ui.ImageSource),
		SettingsBaseURL:    deps.Config.DeepSeekBaseURL,
		SettingsAPIKey:     deps.Config.DeepSeekAPIKey,
		SettingsModel:      deps.Config.DeepSeekModel,
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
	}

	a.OutlineState.List.Key = func(row int) any { return a.OutlineState.Item(row) }

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
	a.OutlineState.Generation++

	if a.SelectedID == "" || func() bool { _, ok := byID[a.SelectedID]; return !ok }() {
		if len(nodes) > 0 {
			a.SelectedID = nodes[0].ID
		} else {
			a.SelectedID = ""
		}
	}
	if a.CollisionTargetID == "" || func() bool { _, ok := byID[a.CollisionTargetID]; return !ok }() {
		a.CollisionTargetID = a.SelectedID
	}
	if a.ShareNodeID == "" || func() bool { _, ok := byID[a.ShareNodeID]; return !ok }() {
		a.ShareNodeID = a.SelectedID
	}

	a.refreshSelectedDetails()

	if slices, err := a.Deps.Store.ListRoamSlices(); err == nil {
		a.RoamSlices = slices
	}
	if insights, err := a.Deps.Store.ListActiveSleepInsights(); err == nil {
		a.SleepInsights = insights
	}
	a.DashboardDigest = a.Deps.Organize.BuildDashboardDigest(a.Nodes, a.EvCounts, a.SleepInsights)
	return nil
}

// returnToDashboard transitions smoothly from the Archive Vault back to the Home Dashboard.
func (a *App) returnToDashboard() {
	a.CaptureDialOpen = false
	a.ActiveSpace = SpaceDashboard
}

// enterArchiveWithTag transitions smoothly from Dashboard into the Mind Palace with an optional tag filter.
func (a *App) enterArchiveWithTag(tag string) {
	a.CaptureDialOpen = false
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
	a.CaptureDialOpen = false
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
	a.Globe.RippleTheme = node.Theme
}

// openNodeDetail opens the selected node in Detail Sheet (phone) or Archive Master-Detail split (desktop).
func (a *App) openNodeDetail(c *ui.Context, id string) {
	a.CaptureDialOpen = false
	a.SelectNode(id)
	phone := true
	if c != nil {
		phone = a.isPhoneLayout(c)
	}
	if phone {
		a.DetailSheetOpen = true
	} else {
		a.ActiveSpace = SpaceArchive
		if a.ArchiveMode == ArchiveRoam {
			a.ArchiveMode = ArchiveCards
		}
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

// SelectNode switches the active detail node and reloads its immutable raw evidences & snapshots.
func (a *App) SelectNode(id string) {
	a.SelectedID = id
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
				a.setErrorBanner(err.Error())
			}
			_ = a.ReloadVault()
		})
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
	a.BannerShowAI = strings.Contains(msg, "DeepSeek API Key")
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
		ui.Column(c).Fill().Background(pal.Canvas).Children(func() {
			a.renderBanner(c, pal)

			if a.QuickOrbOverlayOpen {
				a.renderInAppQuickOrbBar(c, pal)
			}

			a.renderSpaceDeck(c, pal)
		})

		// Mobile-friendly bottom sheets & drawers
		a.renderDetailSheet(c)
		a.renderCaptureModal(c)
		a.renderEvidenceModal(c)
		a.renderSettingsModal(c)
		a.renderSleepModal(c)
		a.renderShareModal(c)
	})
}

func (a *App) renderBanner(c *ui.Context, pal SurfacePalette) {
	if a.BusyTask != "" && a.BannerText == "" {
		ui.Row(c).FillWidth().Padding(8, 12).Margin(10, 16, 0, 16).Gap(8).AlignItems(ui.Center).
			Radius(10).Background(pal.Card).Border(1, pal.Border).Children(func() {
			ui.Spinner(c).Size(14, 14)
			ui.Text(c, a.BusyTask).FontSize(12).TextColor(pal.InkSecondary).Grow(1)
		})
		return
	}
	if a.BannerText == "" {
		return
	}

	th := c.Theme()
	bg := th.Success.Alpha(0.10)
	border := th.Success.Alpha(0.32)
	fg := th.Success
	if a.BannerIsErr {
		bg = th.Danger.Alpha(0.10)
		border = th.Danger.Alpha(0.35)
		fg = th.Danger
	}
	ui.Row(c).FillWidth().Padding(8, 12).Margin(10, 16, 0, 16).Gap(8).AlignItems(ui.Center).
		Radius(10).Background(bg).Border(1, border).
		Children(func() {
			ui.Text(c, a.BannerText).FontSize(12).TextColor(fg).Grow(1).MinWidth(0)
			if a.BannerShowAI {
				if PillAction(c, "banner_ai_cfg", IconSliders, "去设置", true).Clicked() {
					a.SettingsModalOpen = true
				}
			}
			if GhostIconAction(c, "banner_close", IconClose, "关闭提示", false).Clicked() {
				a.BannerText = ""
				a.BannerShowAI = false
			}
		})
}

func (a *App) renderInAppQuickOrbBar(c *ui.Context, pal SurfacePalette) {
	ui.Row(c).FillWidth().Padding(8, 12).Margin(10, 16, 0, 16).Gap(8).AlignItems(ui.Center).
		Radius(14).Background(pal.Card).
		Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.05)).
		Children(func() {
			ui.Icon(c, IconBulb).Size(17, 17).TextColor(ColorsForTheme(domain.ThemeAmber).BadgeText).Shrink(0)
			ui.TextInput(c, &a.FloatQuickText).Placeholder("随手记一句…").Grow(1)
			if PillAction(c, "quick_orb_submit", IconPlus, "记下来", true).Clicked() && strings.TrimSpace(a.FloatQuickText) != "" {
				a.submitFloatingCapture("")
				a.QuickOrbOverlayOpen = false
			}
		})
}
