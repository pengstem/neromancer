package ui

import (
	"context"
	"fmt"
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
	SpaceShowcase Space = iota // 星海展示页
	SpaceMind                  // 思维空间
	SpaceRoam                  // 漫游与海报分享
)

// OrgMode controls how ideas are organized inside Mind Space.
type OrgMode int

const (
	OrgOutline OrgMode = iota // 目录树
	OrgGraph                  // 关联星图
	OrgCards                  // 场景轻卡片
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
	ActiveSpace       Space
	MindOrgMode       OrgMode
	ActiveTag         string
	SearchQuery       string
	DarkMode          bool
	FocusedShowcaseID string
	GraphOffset       ui.Point
	AnimOrigin        time.Time

	// Cached vault state (reloaded cleanly on state mutations)
	Nodes             []domain.IdeaNode
	NodeByID          map[string]domain.IdeaNode
	EvCounts          map[string]int
	Roots             []string
	Children          map[string][]string
	Tags              []service.TagCount
	SelectedID        string
	SelectedEvidences []domain.RawEvidence
	SelectedSnapshots []domain.StageSnapshot
	RoamSlices        []domain.RoamSlice
	SleepInsights     []domain.SleepInsight
	RecallHits        []domain.RecallMatch

	// Interactive widget states kept across frames
	OutlineState    ui.OutlineState[string]
	OutlineChoice   string
	ShowcaseScroll  ui.ScrollState
	ShowcaseRefresh ui.RefreshState
	DetailScroll    ui.ScrollState
	MarkdownOpts    markdown.Options

	// Showcase AI spark
	ShowcaseSpark string

	// Mobile & Desktop Sheets / Drawers
	DetailSheetOpen     bool
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
		ActiveSpace:        SpaceShowcase,
		MindOrgMode:        OrgOutline,
		CaptureModalityIdx: 1, // Quick text/voice dual-ready
		CaptureAutoOrg:     true,
		NodeByID:           make(map[string]domain.IdeaNode),
		EvCounts:           make(map[string]int),
		Children:           make(map[string][]string),
		imgCache:           make(map[string]ui.ImageSource),
		SettingsBaseURL:    deps.Config.DeepSeekBaseURL,
		SettingsAPIKey:     deps.Config.DeepSeekAPIKey,
		SettingsModel:      deps.Config.DeepSeekModel,
		ShowcaseSpark:      "试着把刚路过看到的色彩，和你正在学的一个抽象概念放进同一个比喻里。",
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

// Palette returns the active Day Paper or Night Observatory surface palette.
func (a *App) Palette() SurfacePalette {
	return PaletteForMode(a.DarkMode)
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
	return nil
}

func (a *App) refreshSelectedDetails() {
	if a.SelectedID == "" {
		a.SelectedEvidences = nil
		a.SelectedSnapshots = nil
		return
	}
	if evs, err := a.Deps.Store.ListEvidencesByNode(a.SelectedID); err == nil {
		a.SelectedEvidences = evs
	}
	if snaps, err := a.Deps.Store.ListSnapshotsByNode(a.SelectedID); err == nil {
		a.SelectedSnapshots = snaps
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

// RootView builds the Android-first responsive UI frame-by-frame.
func (a *App) RootView(c *ui.Context) {
	a.FlushPending()
	a.mu.Lock()
	a.Services = c.Services()
	a.mu.Unlock()

	pal := a.Palette()
	phase := a.Phase(c)

	ui.Themed(c, NeromancerThemeFor(a.DarkMode)).Fill().Children(func() {
		ui.Column(c).Fill().Background(pal.Canvas).Children(func() {
			a.renderTopBar(c, pal, phase)
			a.renderBanner(c, pal)

			if a.QuickOrbOverlayOpen {
				a.renderInAppQuickOrbBar(c, pal, phase)
			}

			ui.Box(c).Grow(1).FillWidth().Clip().Children(func() {
				switch a.ActiveSpace {
				case SpaceShowcase:
					a.renderShowcaseView(c)
				case SpaceMind:
					a.renderMindSpaceView(c)
				case SpaceRoam:
					a.renderRoamShareView(c)
				}
			})
		})

		a.renderBottomDock(c, pal)

		// Mobile-friendly bottom sheets & drawers
		a.renderDetailSheet(c)
		a.renderCaptureModal(c)
		a.renderEvidenceModal(c)
		a.renderSettingsModal(c)
		a.renderSleepModal(c)
		a.renderShareModal(c)
	})
}

func (a *App) renderTopBar(c *ui.Context, pal SurfacePalette, phase float32) {
	ui.Row(c).Height(48).FillWidth().Padding(0, 14).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(22, 22).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				DrawAnimatedMemoryOrb(p, r, domain.ThemeCoral, 0.9, phase, 0.25, "petals", false)
			})
			ui.Text(c, "Neromancer").FontSize(16).Bold().TextColor(pal.Ink).SingleLine()
			ui.Text(c, fmt.Sprintf("%d", len(a.Nodes))).FontSize(11).Bold().
				Padding(2, 7).Radius(999).
				Background(pal.SoftPanel).TextColor(pal.InkMuted).SingleLine()
		})

		ui.Spacer(c).Grow(1)

		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if a.BusyTask != "" {
				ui.Spinner(c).Size(14, 14)
			}

			darkIcon := IconMoon
			if a.DarkMode {
				darkIcon = IconSun
			}
			if IconAction(c, "top_dark", darkIcon, "夜间模式", a.DarkMode).Tooltip("切换夜间模式").Clicked() {
				mygo.Haptics.Selection()
				a.DarkMode = !a.DarkMode
			}

			if IconAction(c, "top_float", IconFloat, "悬浮速记", a.QuickOrbOverlayOpen).Tooltip("悬浮速记").Clicked() {
				if a.FloatOnToggle != nil {
					a.FloatOnToggle()
				} else {
					a.QuickOrbOverlayOpen = !a.QuickOrbOverlayOpen
				}
			}

			ui.Box(c).Key("top_sleep_wrap").Size(34, 34).Shrink(0).Children(func() {
				if IconAction(c, "top_sleep", IconSpark, "睡眠孵化", a.SleepModalOpen).Tooltip("睡眠孵化").Clicked() {
					a.SleepModalOpen = true
				}
				if len(a.SleepInsights) > 0 {
					ui.Box(c).Absolute().Top(4).Right(4).Size(6, 6).Radius(3).
						Background(c.Theme().Danger).PassThrough()
				}
			})

			configured := a.Deps.AI != nil && a.Deps.AI.Configured()
			ui.Box(c).Key("top_settings_wrap").Size(34, 34).Shrink(0).Children(func() {
				if IconAction(c, "top_settings", IconSliders, "DeepSeek 设置", a.SettingsModalOpen).Tooltip("DeepSeek 设置").Clicked() {
					a.SettingsModalOpen = true
				}
				if configured {
					ui.Box(c).Absolute().Top(4).Right(4).Size(6, 6).Radius(3).
						Background(c.Theme().Success).PassThrough()
				}
			})
		})
	})
}

func (a *App) renderBottomDock(c *ui.Context, pal SurfacePalette) {
	if a.DetailSheetOpen || a.CaptureModalOpen || a.EvidenceModalOpen || a.SettingsModalOpen || a.SleepModalOpen || a.ShareModalOpen {
		return
	}
	_, _, bottomSafe, _ := c.SafeArea()
	ui.Overlay(c, func() {
		ui.Row(c).Key("bottom_dock").Absolute().Left(14).Right(14).Bottom(14 + bottomSafe).
			Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).PassThrough().
			Children(func() {
				a.renderSpaceDockCapsule(c, pal)
				a.renderMorphingFAB(c)
			})
	})
}

func (a *App) renderSpaceDockCapsule(c *ui.Context, pal SurfacePalette) {
	tabs := []struct {
		space Space
		icon  *ui.SVG
		label string
	}{
		{SpaceShowcase, IconOrb, "星海"},
		{SpaceMind, IconMind, "思维空间"},
		{SpaceRoam, IconRoam, "漫游分享"},
	}

	ui.Row(c).Key("dock_capsule").Padding(4).Gap(2).Radius(999).AlignItems(ui.Center).Shrink(0).
		Shadow(0, 8, 22, 0, ui.RGBA(0, 0, 0, 0.14)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			DrawFrostSurface(p, r, pal.GlassTint, pal.GlassBorder, 999, 16)
		}).
		Clip().
		Children(func() {
			for _, tb := range tabs {
				tab := tb
				active := a.ActiveSpace == tab.space
				btn := ui.ButtonBase(c).Key("dock_tab_"+tab.label).
					Padding(6, 9).Gap(4).Radius(999).Center().
					Cursor(ui.CursorPointer).Shrink(0).Clip().Label(tab.label)
				if active {
					btn.Background(pal.Accent).TextColor(pal.AccentText)
				} else {
					bg := ui.Transparent
					if btn.Pressed() {
						bg = pal.SoftPanel.Alpha(0.75)
					} else if btn.Hovered() {
						bg = pal.SoftPanel.Alpha(0.45)
					}
					btn.Background(bg).TextColor(pal.InkSecondary)
				}
				mobile.Ink(btn)
				btn.Children(func() {
					ui.Icon(c, tab.icon).Size(13.5, 13.5)
					ui.Text(c, tab.label).FontSize(11.5).Bold().SingleLine()
				})
				if btn.Clicked() {
					mygo.Haptics.Selection()
					a.ActiveSpace = tab.space
				}
			}
		})
}

func (a *App) renderBanner(c *ui.Context, pal SurfacePalette) {
	if a.BusyTask != "" && a.BannerText == "" {
		ui.Row(c).FillWidth().Padding(8, 12).Margin(0, 14, 6, 14).Gap(8).AlignItems(ui.Center).
			Radius(10).Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
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
	ui.Row(c).FillWidth().Padding(8, 12).Margin(0, 14, 6, 14).Gap(8).AlignItems(ui.Center).
		Radius(10).Background(bg).Border(1, border).
		Children(func() {
			ui.Text(c, a.BannerText).FontSize(12).TextColor(fg).Grow(1)
			if a.BannerShowAI {
				if PillAction(c, "banner_ai_cfg", IconSliders, "配置接口", true).Clicked() {
					a.SettingsModalOpen = true
				}
			}
			if IconAction(c, "banner_close", IconClose, "关闭提示", false).Clicked() {
				a.BannerText = ""
				a.BannerShowAI = false
			}
		})
}

func (a *App) renderInAppQuickOrbBar(c *ui.Context, pal SurfacePalette, phase float32) {
	ui.Row(c).FillWidth().Padding(8, 12).Margin(0, 14, 8, 14).Gap(8).AlignItems(ui.Center).
		Radius(14).Background(pal.Card).Border(1, pal.Border).
		Children(func() {
			ui.Box(c).Size(24, 24).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
				DrawAnimatedMemoryOrb(p, r, domain.ThemeCoral, 0.9, phase, 0.35, "petals", true)
			})
			ui.TextInput(c, &a.FloatQuickText).Placeholder("记下此刻闪念…").Grow(1)
			if PillAction(c, "quick_orb_submit", IconPlus, "收入星海", true).Clicked() && strings.TrimSpace(a.FloatQuickText) != "" {
				a.submitFloatingCapture("")
				a.QuickOrbOverlayOpen = false
			}
		})
}
