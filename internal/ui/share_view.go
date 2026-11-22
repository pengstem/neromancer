package ui

import (
	"fmt"
	"image"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

// renderPosterArtCard renders the unified Bento Gallery Emblem poster layout shared across
// the share sheet modal and 1200x1640 PNG export.
func renderPosterArtCard(c *ui.Context, pal SurfacePalette, node domain.IdeaNode, evCount int, exportMode bool) {
	orbPal := ColorsForTheme(node.Theme)
	softTint, badgeBg, badgeText := mindOrbSurface(pal, node.Theme)
	iconKey := domain.ResolveIdeaIconKey(node)

	pad := float32(18)
	gap := float32(12)
	emblemInner := float32(68)
	emblemIcon := float32(34)
	titleSize := float32(17)
	bodySize := float32(13)
	maxSummaryLines := 4
	if exportMode {
		pad = 32
		gap = 16
		emblemInner = 122
		emblemIcon = 60
		titleSize = 25
		bodySize = 15
		maxSummaryLines = 6
	}

	card := ui.Column(c).Padding(pad).Gap(gap).Radius(16).
		LinearGradient(ui.LinearGradient{
			From:  pal.Card,
			To:    softTint,
			Angle: 180,
			Oklab: true,
		})
	if exportMode {
		card.Fill()
	} else {
		card.FillWidth()
	}

	card.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
			ui.Text(c, "NEROMANCER").
				FontSize(9.5).FontWeight(700).LetterSpacing(1.6).
				TextColor(badgeText).SingleLine().MinWidth(0)
			if folder := domain.ResolveIdeaFolder(node); folder != "" {
				ui.Text(c, folder).FontSize(10).FontWeight(500).TextColor(pal.InkMuted).SingleLine()
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.85))

		if exportMode {
			ui.Spacer(c).Grow(1)
		}

		ui.Column(c).FillWidth().Gap(gap).AlignItems(ui.Center).Children(func() {
			IdeaIconBadge(c, iconKey, node.Theme, IconBadgeProps{
				Size:     emblemInner,
				IconSize: emblemIcon,
				Radius:   emblemInner * 0.5,
			})

			ui.Text(c, node.Title).
				FontSize(titleSize).Bold().LineHeight(1.32).
				TextColor(pal.Ink).TextAlign(ui.Center).Balance()

			ui.Text(c, node.ConversationalSummary).
				FontSize(bodySize).LineHeight(1.64).
				TextColor(pal.InkSecondary).TextAlign(ui.Center).Pretty().MaxLines(maxSummaryLines)

			if len(node.Tags) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(6).Center().Children(func() {
					shown := 0
					for i, rawTag := range node.Tags {
						tag := cleanTagLabel(rawTag)
						if tag == "" {
							continue
						}
						ui.Text(c, tag).Key(fmt.Sprintf("poster_tag_%s_%d", node.ID, i)).
							FontSize(10.5).FontWeight(500).Padding(3, 9).Radius(6).
							Background(badgeBg.Alpha(0.82)).TextColor(badgeText)
						shown++
						if shown >= 4 {
							break
						}
					}
				})
			}

			if capText := strings.TrimSpace(node.Visual.Caption); capText != "" {
				ui.Text(c, capText).FontSize(11).Italic().TextColor(orbPal.BadgeText.Alpha(0.75)).TextAlign(ui.Center)
			}
		})

		if exportMode {
			ui.Spacer(c).Grow(1)
		}

		ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.85))

		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Textf(c, "%d 条原记录", max(1, evCount)).FontSize(10.5).TextColor(pal.InkMuted)
			ui.Text(c, formatPosterDate(node.UpdatedAt)).FontSize(10.5).FontFeatures("tnum").TextColor(pal.InkMuted)
		})
	})
}

func (a *App) exportOrbCapsule(nodeID string) {
	p, err := a.Deps.Share.ExportCapsule(nodeID)
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	mygo.Haptics.Success()
	a.LastExportedPath = p
	a.setBanner("已导出灵感备份：" + p)
}

func (a *App) exportAndMaybeSharePNG(node domain.IdeaNode, openShareSheet bool) {
	p, err := a.Deps.Share.ExportPNGCard(node.ID)
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	mygo.Haptics.Success()
	a.LastExportedPath = p
	if openShareSheet {
		if err := mygo.Share(mygo.ShareOptions{
			Text:  fmt.Sprintf("分享一条灵感：《%s》", node.Title),
			Files: []string{p},
		}); err != nil {
			a.setBanner("海报已保存到本地：" + p)
			return
		}
	}
	a.setBanner("已导出分享海报：" + p)
}

func (a *App) renderShareModal(c *ui.Context) {
	pal := a.Palette()

	node, ok := a.NodeByID[a.ShareNodeID]
	if !ok {
		if sel, selOK := a.SelectedNode(); selOK {
			node = sel
			ok = true
		} else {
			a.ShareModalOpen = false
		}
	}

	sheetBg := pal.Card
	mobile.Sheet(c, &a.ShareModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.76, 0.94},
		Background: &sheetBg,
		Label:      "分享灵感",
	}, func() {
		if !ok {
			return
		}
		evCount := a.EvCounts[node.ID]

		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "分享灵感").FontSize(17).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0)
					if GhostIconAction(c, "share_modal_close", IconClose, "关闭", false).Clicked() {
						a.ShareModalOpen = false
					}
				})

				ui.Box(c).FillWidth().Radius(16).
					Shadow(0, 4, 16, 0, ui.RGBA(18, 22, 33, 0.06)).
					Children(func() {
						renderPosterArtCard(c, pal, node, evCount, false)
					})

				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
					if PillAction(c, "share_modal_capsule", IconArchive, "导出备份", false).Clicked() {
						a.ShareModalOpen = false
						a.exportOrbCapsule(node.ID)
					}
					if PillAction(c, "share_modal_save", IconDownload, "保存海报", false).Clicked() {
						a.ShareModalOpen = false
						a.exportAndMaybeSharePNG(node, false)
					}
					if PillAction(c, "share_modal_poster", IconShare, "系统分享", true).Clicked() {
						a.ShareModalOpen = false
						a.exportAndMaybeSharePNG(node, runtime.GOOS == "android" || runtime.GOOS == "ios" || runtime.GOOS == "darwin")
					}
				})
			})
		})
	})
}

func (a *App) importCapsuleAsync() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  a.MainWin,
			Title:   "选择灵感备份文件 (.orb.json)",
			Filters: []mygo.FileFilter{{Name: "Orb Backup", Extensions: []string{"json"}}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		node, err := a.Deps.Share.ImportCapsuleFile(paths[0])
		a.Dispatch(func() {
			if err != nil {
				a.setErrorBanner(err.Error())
				return
			}
			_ = a.ReloadVault()
			a.SelectNode(node.ID)
			a.ActiveSpace = SpaceArchive
			a.DetailSheetOpen = true
			a.setBanner("已导入灵感：《" + node.Title + "》")
		})
	}()
}

// RenderSharePoster renders a 600x820 @ 2x (1200x1640) museum-grade archival PNG exhibition poster.
func RenderSharePoster(node domain.IdeaNode, evCount int) *image.RGBA {
	pal := DefaultPalette()
	return ui.Render(func(c *ui.Context) {
		ui.Themed(c, NeromancerTheme()).Fill().Children(func() {
			ui.Box(c).Fill().Padding(28).Background(pal.Canvas).Children(func() {
				renderPosterArtCard(c, pal, node, evCount, true)
			})
		})
	}, 600, 820, 2)
}

func formatPosterDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}

type selectOption struct {
	Value string
	Label string
}

func atelierSelect(c *ui.Context, pal SurfacePalette, key, label string, selected *string, opts []selectOption) bool {
	if len(opts) == 0 {
		return false
	}
	curLabel := ""
	for _, opt := range opts {
		if opt.Value == *selected {
			curLabel = opt.Label
			break
		}
	}
	if curLabel == "" {
		curLabel = opts[0].Label
	}

	sel := ui.SelectBase(c.Key(key), selected)
	sel.Trigger.
		Grow(1).MinWidth(0).
		Padding(8, 12).Gap(8).Radius(10).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Background(pal.Card).Border(1, pal.Border).
		Cursor(ui.CursorPointer).Label(label)
	mobile.Ink(sel.Trigger)
	sel.Trigger.Children(func() {
		ui.Text(c, curLabel).FontSize(13).TextColor(pal.Ink).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, IconChevronDown).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
	})
	sel.Popup(func(panel ui.Element) {
		panel.Margin(4, 0, 0, 0).Padding(4).Radius(12).MaxHeight(240).
			Background(pal.Card).Border(1, pal.Border).
			Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, 0.16))
		ui.Scroll(c).Shrink(1).Children(func() {
			for i, opt := range opts {
				o := opt
				item := sel.Item(o.Value).Key(fmt.Sprintf("%s_opt_%d", key, i)).
					Padding(8, 10).Radius(8).Cursor(ui.CursorPointer)
				switch {
				case item.Highlighted():
					item.Background(pal.Ink).TextColor(pal.Card)
				case o.Value == *selected:
					item.Background(pal.SoftPanel).TextColor(pal.Ink)
				default:
					item.TextColor(pal.Ink)
				}
				item.Children(func() {
					ui.Text(c, o.Label).FontSize(13).SingleLine().MinWidth(0)
				})
			}
		})
	})
	return sel.Trigger.Changed()
}
