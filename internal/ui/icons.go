package ui

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

// cleanTagLabel strips any legacy leading '#' / '＃' and surrounding spaces so tags never show raw hash syntax.
func cleanTagLabel(tag string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(tag), "#＃"))
}

func mustIconSVG(inner string) *ui.SVG {
	const head = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round">`
	const tail = `</svg>`
	return ui.MustParseSVG([]byte(head + inner + tail))
}

// Vector studio & semantic archive icons compiled once at startup.
var (
	IconOrb           = mustIconSVG(`<circle cx="12" cy="12" r="9"/><path d="M8.5 9.5a5 5 0 0 1 5-3.2"/><circle cx="15.5" cy="15.5" r="1.2"/>`)
	IconMind          = mustIconSVG(`<circle cx="12" cy="5.5" r="2.5"/><circle cx="6" cy="17.5" r="2.5"/><circle cx="18" cy="17.5" r="2.5"/><path d="M10.8 7.7 7.2 15.3"/><path d="M13.2 7.7 16.8 15.3"/><path d="M8.5 17.5h7"/>`)
	IconRoam          = mustIconSVG(`<circle cx="9.5" cy="12" r="6"/><circle cx="14.5" cy="12" r="6"/>`)
	IconMic           = mustIconSVG(`<rect x="9" y="3" width="6" height="11" rx="3"/><path d="M5 11a7 7 0 0 0 14 0"/><line x1="12" y1="18" x2="12" y2="21"/><line x1="9" y1="21" x2="15" y2="21"/>`)
	IconStop          = mustIconSVG(`<rect x="6" y="6" width="12" height="12" rx="2.5"/>`)
	IconPlay          = mustIconSVG(`<polygon points="7 5 19 12 7 19 7 5"/>`)
	IconQuill         = mustIconSVG(`<path d="M20.2 3.8a4.2 4.2 0 0 0-5.9 0L5.5 12.6 4 20l7.4-1.5 8.8-8.8a4.2 4.2 0 0 0 0-5.9Z"/><path d="M16 8 4 20"/><path d="M17.5 13H11"/>`)
	IconCamera        = mustIconSVG(`<path d="M4 7h3l2-2.5h6L17 7h3a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V9a2 2 0 0 1 2-2Z"/><circle cx="12" cy="13" r="3.5"/>`)
	IconImage         = mustIconSVG(`<rect x="3" y="4" width="18" height="16" rx="2.5"/><circle cx="8.5" cy="9.5" r="1.5"/><path d="m21 16-5-5-6 6-2.5-2.5L3 19"/>`)
	IconPlus          = mustIconSVG(`<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>`)
	IconClose         = mustIconSVG(`<line x1="6" y1="6" x2="18" y2="18"/><line x1="18" y1="6" x2="6" y2="18"/>`)
	IconCheck         = mustIconSVG(`<polyline points="5 12.5 10 17.5 19 7.5"/>`)
	IconOutline       = mustIconSVG(`<line x1="4" y1="6" x2="20" y2="6"/><line x1="9" y1="12" x2="20" y2="12"/><line x1="9" y1="18" x2="20" y2="18"/><circle cx="5" cy="12" r="1"/><circle cx="5" cy="18" r="1"/>`)
	IconCards         = mustIconSVG(`<rect x="3.5" y="4" width="7.2" height="7.2" rx="2"/><rect x="13.3" y="4" width="7.2" height="7.2" rx="2"/><rect x="3.5" y="13" width="7.2" height="7.2" rx="2"/><rect x="13.3" y="13" width="7.2" height="7.2" rx="2"/>`)
	IconConstellation = mustIconSVG(`<circle cx="6" cy="16" r="1.8"/><circle cx="10.5" cy="7.5" r="1.8"/><circle cx="18" cy="10" r="1.8"/><circle cx="15.5" cy="17.5" r="1.8"/><path d="M7.1 14.5 9.5 9.1"/><path d="M12.2 8.1 16.3 9.4"/><path d="M17.4 11.7 16.1 15.8"/>`)
	IconFilmstrip     = mustIconSVG(`<rect x="3" y="5" width="18" height="14" rx="2"/><line x1="7.5" y1="5" x2="7.5" y2="19"/><line x1="16.5" y1="5" x2="16.5" y2="19"/><line x1="3" y1="9.5" x2="7.5" y2="9.5"/><line x1="3" y1="14.5" x2="7.5" y2="14.5"/><line x1="16.5" y1="9.5" x2="21" y2="9.5"/><line x1="16.5" y1="14.5" x2="21" y2="14.5"/>`)
	IconArchive       = mustIconSVG(`<rect x="3" y="4" width="18" height="4.5" rx="1"/><path d="M5 8.5v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-10"/><line x1="10" y1="13" x2="14" y2="13"/>`)
	IconShare         = mustIconSVG(`<path d="M12 3v12"/><polyline points="7.5 7.5 12 3 16.5 7.5"/><path d="M5 14v5a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-5"/>`)
	IconDownload      = mustIconSVG(`<path d="M12 3v12"/><polyline points="7.5 10.5 12 15 16.5 10.5"/><path d="M5 16v3a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-3"/>`)
	IconUpload        = mustIconSVG(`<path d="M12 15V3"/><polyline points="7.5 7.5 12 3 16.5 7.5"/><path d="M4 16v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3"/>`)
	IconSpark         = mustIconSVG(`<path d="m12 3 1.9 5.6L19.5 10.5l-5.6 1.9L12 18l-1.9-5.6L4.5 10.5l5.6-1.9Z"/><path d="M18.5 16.5v3"/><path d="M17 18h3"/>`)
	IconRefresh       = mustIconSVG(`<path d="M20 11a8 8 0 0 0-14.9-3M4 4v4h4"/><path d="M4 13a8 8 0 0 0 14.9 3M20 20v-4h-4"/>`)
	IconCollide       = mustIconSVG(`<path d="M13 3 5 14h6l-1 7 9-11h-6V3Z"/>`)
	IconSearch        = mustIconSVG(`<circle cx="11" cy="11" r="7"/><line x1="20" y1="20" x2="16.2" y2="16.2"/>`)
	IconMoon          = mustIconSVG(`<path d="M20 14.5A8.5 8.5 0 0 1 9.5 4a8.5 8.5 0 1 0 10.5 10.5Z"/>`)
	IconSliders       = mustIconSVG(`<line x1="4" y1="7" x2="7" y2="7"/><line x1="11.5" y1="7" x2="20" y2="7"/><circle cx="9.25" cy="7" r="2.25"/><line x1="4" y1="17" x2="12.5" y2="17"/><line x1="17" y1="17" x2="20" y2="17"/><circle cx="14.75" cy="17" r="2.25"/>`)
	IconFloat         = mustIconSVG(`<rect x="3" y="4" width="18" height="16" rx="2.5"/><circle cx="15.5" cy="13.5" r="3"/>`)
	IconChevronDown   = mustIconSVG(`<polyline points="6 9 12 15 18 9"/>`)
	IconChevronUp     = mustIconSVG(`<polyline points="6 15 12 9 18 15"/>`)
	IconChevronRight  = mustIconSVG(`<polyline points="9 6 15 12 9 18"/>`)
	IconMapPin        = mustIconSVG(`<path d="M12 21s-6-5.4-6-10.5a6 6 0 1 1 12 0C18 15.6 12 21 12 21Z"/><circle cx="12" cy="10.5" r="2"/>`)
	IconClock         = mustIconSVG(`<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15.5 14"/>`)
	IconArrowDown     = mustIconSVG(`<line x1="12" y1="5" x2="12" y2="19"/><polyline points="6 13 12 19 18 13"/>`)
	IconArrowUp       = mustIconSVG(`<line x1="12" y1="19" x2="12" y2="5"/><polyline points="6 11 12 5 18 11"/>`)
	IconArchiveFolder = mustIconSVG(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"/>`)

	// Semantic auto-matched icons for Inspiration Archive entries.
	IconBotanical = mustIconSVG(`<path d="M12 21v-8"/><path d="M12 13c-3.5 0-6-2.5-6-6.5 0-1.5.5-2.8 1.2-3.5 1.6.6 3 1.8 3.6 3.3.4-1.6 1.2-2.8 2.2-3.3 1 .5 1.8 1.7 2.2 3.3.6-1.5 2-2.7 3.6-3.3.7.7 1.2 2 1.2 3.5 0 4-2.5 6.5-6 6.5Z"/><path d="M12 17c2.2 0 4-1.3 4.8-3.2"/><path d="M12 18c-2.2 0-4-1.3-4.8-3.2"/>`)
	IconCompass   = mustIconSVG(`<circle cx="12" cy="12" r="9"/><polygon points="15.5 8.5 13.5 13.5 8.5 15.5 10.5 10.5 15.5 8.5"/>`)
	IconWave      = mustIconSVG(`<line x1="4" y1="11" x2="4" y2="13"/><line x1="8" y1="8" x2="8" y2="16"/><line x1="12" y1="5" x2="12" y2="19"/><line x1="16" y1="9" x2="16" y2="15"/><line x1="20" y1="11" x2="20" y2="13"/>`)
	IconPrism     = mustIconSVG(`<path d="M12 3 3.5 18.5h17L12 3Z"/><path d="M12 3v15.5"/><path d="m12 11 8.5 7.5"/>`)
	IconCode      = mustIconSVG(`<polyline points="8 7 3 12 8 17"/><polyline points="16 7 21 12 16 17"/><line x1="13.5" y1="5" x2="10.5" y2="19"/>`)
	IconBook      = mustIconSVG(`<path d="M2 5.5A2.5 2.5 0 0 1 4.5 3H10a2 2 0 0 1 2 2v15a1.5 1.5 0 0 0-1.5-1.5H4.5A2.5 2.5 0 0 1 2 16V5.5Z"/><path d="M22 5.5A2.5 2.5 0 0 0 19.5 3H14a2 2 0 0 0-2 2v15a1.5 1.5 0 0 1 1.5-1.5h6A2.5 2.5 0 0 0 22 16V5.5Z"/>`)
	IconMusic     = mustIconSVG(`<path d="M9 18V6l11-2v12"/><circle cx="6.5" cy="18" r="2.5"/><circle cx="17.5" cy="16" r="2.5"/>`)
	IconCoffee    = mustIconSVG(`<path d="M17 8h1.5a3.5 3.5 0 0 1 0 7H17"/><path d="M4 8h13v6a5 5 0 0 1-5 5H9a5 5 0 0 1-5-5V8Z"/><line x1="7" y1="3" x2="7" y2="5"/><line x1="11" y1="3" x2="11" y2="5"/>`)
	IconBulb      = mustIconSVG(`<path d="M9 18h6"/><path d="M10 21h4"/><path d="M15.5 14c.8-.9 1.5-2.1 1.5-3.8a5 5 0 1 0-10 0c0 1.7.7 2.9 1.5 3.8.6.7 1 1.3 1 2h5c0-.7.4-1.3 1-2Z"/>`)
)

// SVGForIconKey returns the vector SVG matching a canonical or legacy icon key.
func SVGForIconKey(iconKey string) *ui.SVG {
	switch domain.NormalizeIconKey(iconKey) {
	case domain.IconKeyBotanical:
		return IconBotanical
	case domain.IconKeyCompass:
		return IconCompass
	case domain.IconKeyWave:
		return IconWave
	case domain.IconKeyPrism:
		return IconPrism
	case domain.IconKeyCode:
		return IconCode
	case domain.IconKeyBook:
		return IconBook
	case domain.IconKeyMusic:
		return IconMusic
	case domain.IconKeyCamera:
		return IconCamera
	case domain.IconKeyCoffee:
		return IconCoffee
	default:
		return IconBulb
	}
}

// ResolveNodeSVG resolves the semantic vector SVG icon for an IdeaNode.
func ResolveNodeSVG(node domain.IdeaNode) *ui.SVG {
	return SVGForIconKey(domain.ResolveIdeaIconKey(node))
}

// IconBadgeProps configures IdeaIconBadge dimensions; zero values produce the 38px archive tile badge.
type IconBadgeProps struct {
	Size     float32
	IconSize float32
	Radius   float32
}

// IdeaIconBadge renders a tinted rounded badge housing an auto-matched semantic SVG icon.
func IdeaIconBadge(c *ui.Context, iconKey string, theme domain.OrbTheme, p IconBadgeProps) ui.Element {
	size := p.Size
	if size <= 0 {
		size = 38
	}
	icSize := p.IconSize
	if icSize <= 0 {
		icSize = 19
	}
	rad := p.Radius
	if rad <= 0 {
		rad = 10
	}
	orbPal := ColorsForTheme(theme)
	return ui.Box(c).Size(size, size).Radius(rad).Shrink(0).Center().
		Background(orbPal.BadgeBg).
		Children(func() {
			ui.Icon(c, SVGForIconKey(iconKey)).Size(icSize, icSize).TextColor(orbPal.BadgeText)
		})
}

// IconAction builds a compact 32x32 rounded icon button with accessible label and Material ink.
func IconAction(c *ui.Context, key string, ic *ui.SVG, label string, active bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c)
	if key != "" {
		b.Key(key)
	}

	bg := t.Surface
	hover := t.SurfaceHover
	pressed := t.SurfacePressed
	border := t.Border
	fg := t.TextMuted
	if active {
		const alpha = float32(0.10)
		bg = t.Accent.Alpha(alpha)
		hover = t.Accent.Alpha(alpha + 0.05)
		pressed = t.Accent.Alpha(alpha + 0.10)
		border = t.Accent.Alpha(0.35)
		fg = t.Accent
	}

	face := bg
	if b.Pressed() {
		face = pressed
	} else if b.Hovered() {
		face = hover
	}

	b.Size(32, 32).Radius(RadiusControl).Center().
		Background(face).Border(1, border).TextColor(fg).
		Cursor(ui.CursorPointer).Shrink(0).Clip().Label(label)
	mobile.Ink(b)

	if ic != nil {
		b.Children(func() {
			ui.Icon(c, ic).Size(16, 16)
		})
	}
	return b
}

// GhostIconAction builds a quiet borderless 32x32 icon button for sheet close actions and restrained header tools.
func GhostIconAction(c *ui.Context, key string, ic *ui.SVG, label string, active bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c)
	if key != "" {
		b.Key(key)
	}

	bg := ui.Transparent
	hover := t.SurfaceHover
	pressed := t.SurfacePressed
	fg := t.TextMuted
	if active {
		bg = t.Accent.Alpha(0.10)
		fg = t.Accent
	}

	face := bg
	if b.Pressed() {
		face = pressed
	} else if b.Hovered() {
		face = hover
	}

	b.Size(32, 32).Radius(RadiusControl).Center().
		Background(face).TextColor(fg).
		Cursor(ui.CursorPointer).Shrink(0).Clip().Label(label)
	mobile.Ink(b)

	if ic != nil {
		b.Children(func() {
			ui.Icon(c, ic).Size(16, 16)
		})
	}
	return b
}

// PillAction builds a tactile borderless capsule button pairing a 14px SVG icon and single-line label.
func PillAction(c *ui.Context, key string, ic *ui.SVG, label string, primary bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c)
	if key != "" {
		b.Key(key)
	}

	bg := ColorSecondaryFill
	hover := ColorSecondaryHov
	pressed := ColorSecondaryAct
	fg := t.Text
	if primary {
		bg = t.Accent
		hover = t.AccentHover
		pressed = t.AccentPressed
		fg = t.AccentText
	}

	face := bg
	if b.Pressed() {
		face = pressed
	} else if b.Hovered() {
		face = hover
	}

	b.Padding(7, 12).Gap(6).Radius(RadiusPill).Center().
		Background(face).TextColor(fg).
		Cursor(ui.CursorPointer).Shrink(0).Clip().Label(label)
	mobile.Ink(b)

	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(14, 14)
		}
		if label != "" {
			ui.Text(c, label).FontSize(FontMeta).Bold().SingleLine()
		}
	})
	return b
}

// GhostTextAction builds a borderless, zero-fill inline icon+text action for quiet secondary triggers.
func GhostTextAction(c *ui.Context, key string, ic *ui.SVG, label string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c)
	if key != "" {
		b.Key(key)
	}

	face := ui.Transparent
	if b.Pressed() {
		face = t.SurfacePressed
	} else if b.Hovered() {
		face = t.SurfaceHover
	}

	b.Padding(5, 8).Gap(5).Radius(RadiusControl).Center().
		Background(face).TextColor(t.TextMuted).
		Cursor(ui.CursorPointer).Shrink(0).Clip().Label(label)
	mobile.Ink(b)

	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).Size(14, 14)
		}
		if label != "" {
			ui.Text(c, label).FontSize(FontMeta).FontWeight(600).SingleLine()
		}
	})
	return b
}
