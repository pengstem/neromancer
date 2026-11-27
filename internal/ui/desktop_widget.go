package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

const (
	desktopWidgetWidth  = 320
	desktopWidgetHeight = 92
)

// DesktopWidgetBounds is the panel size in DIPs.
func DesktopWidgetBounds() (width, height int) {
	return desktopWidgetWidth, desktopWidgetHeight
}

type desktopWidgetAction struct {
	key      string
	label    string
	icon     *ui.SVG
	theme    domain.OrbTheme
	modality int
}

var desktopWidgetActions = []desktopWidgetAction{
	{key: "widget_say", label: "说一句", icon: IconMic, theme: domain.ThemeCoral, modality: 0},
	{key: "widget_write", label: "写一句", icon: IconQuill, theme: domain.ThemeSky, modality: 1},
	{key: "widget_snap", label: "拍一张", icon: IconCamera, theme: domain.ThemeAmber, modality: 2},
}

// CaptureLaunchFromURL reports whether rawURL asks to start a capture, and
// which editor to open: 0 voice, 1 text, 2 image. A bare capture link opens
// text.
func CaptureLaunchFromURL(rawURL string) (modality int, ok bool) {
	u := strings.ToLower(strings.TrimSpace(rawURL))
	if !strings.Contains(u, "capture") {
		return 0, false
	}
	switch {
	case strings.Contains(u, "voice"), strings.Contains(u, "audio"), strings.Contains(u, "mic"):
		return 0, true
	case strings.Contains(u, "image"), strings.Contains(u, "photo"), strings.Contains(u, "camera"):
		return 2, true
	default:
		return 1, true
	}
}

// OpenCaptureFromWidget brings the main window forward and opens one capture
// editor. modality is 0 voice, 1 text, 2 image.
func (a *App) OpenCaptureFromWidget(modality int) {
	if modality < 0 || modality > 2 {
		modality = 1
	}
	a.WidgetHidden = false
	a.SettingsModalOpen = false
	a.DetailSheetOpen = false
	a.ShareModalOpen = false
	a.EvidenceModalOpen = false
	a.ActiveSpace = SpaceDashboard
	a.openCaptureModal(modality, ui.Rect{})
	if a.MainWin != nil && !a.MainWin.IsDestroyed() {
		a.MainWin.Show()
		a.MainWin.Focus()
		a.MainWin.Invalidate()
	}
}

// ShowDesktopWidget brings the desktop quick-capture panel back.
func (a *App) ShowDesktopWidget() {
	a.WidgetHidden = false
	if a.FloatWin != nil && !a.FloatWin.IsDestroyed() {
		a.FloatWin.Show()
	}
}

func (a *App) hideDesktopWidget() {
	a.WidgetHidden = true
	if a.FloatWin != nil && !a.FloatWin.IsDestroyed() {
		a.FloatWin.Hide()
	}
}

// DesktopWidgetView is the small always-on-top panel for starting a capture
// without digging through the phone window.
func (a *App) DesktopWidgetView(c *ui.Context) {
	ui.Themed(c, NeromancerTheme()).Fill().Padding(6).Children(func() {
		ui.Row(c).Fill().Padding(8, 10).Gap(6).AlignItems(ui.Center).
			Radius(22).Background(ColorCardBg).Border(1, ColorBorderSoft).
			Shadow(0, 8, 22, 0, ui.RGBA(20, 21, 25, 0.12)).
			DragWindow().
			Children(func() {
				ui.Column(c).Gap(1).Grow(1).Shrink(1).MinWidth(0).Children(func() {
					ui.Text(c, "记一句").FontSize(FontCardTitle).FontWeight(600).TextColor(ColorInkPrimary).SingleLine()
				})
				for _, act := range desktopWidgetActions {
					a.renderDesktopWidgetAction(c, act)
				}
				hideBtn := GhostIconAction(c, "widget_hide", IconClose, "收起", false)
				if hideBtn.Clicked() {
					a.hideDesktopWidget()
				}
			})
	})
}

func (a *App) renderDesktopWidgetAction(c *ui.Context, act desktopWidgetAction) {
	colors := ColorsForTheme(act.theme)
	btn := ui.ButtonBase(c).Key(act.key).Label(act.label)
	face := ui.Transparent
	if btn.Pressed() {
		face = colors.BadgeBg
	} else if btn.Hovered() {
		face = colors.Core
	}
	btn.Column().Width(52).AlignItems(ui.Center).Padding(2, 0).Gap(2).Radius(RadiusControl).
		Background(face).Cursor(ui.CursorPointer).Shrink(0)
	mobile.Ink(btn)
	btn.Children(func() {
		ui.Box(c).Size(28, 28).Radius(14).Center().Background(colors.BadgeBg).Children(func() {
			ui.Icon(c, act.icon).Size(15, 15).TextColor(colors.BadgeText)
		})
		ui.Text(c, act.label).FontSize(FontCaption).FontWeight(500).TextColor(ColorInkSecondary).SingleLine()
	})
	if btn.Clicked() {
		a.OpenCaptureFromWidget(act.modality)
	}
}
