package ui

import (
	"context"
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

type selectOption struct {
	Value string
	Label string
}

func (a *App) renderRoamShareView(c *ui.Context) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)

	padX := float32(16)
	if !phone {
		padX = 24
	}

	ui.Scroll(c).Key("space_roam").TrackScroll(&a.RoamScroll).Grow(1).FillWidth().Background(pal.Canvas).
		HandleInput(func(ev ui.InputEvent) bool {
			return a.handleSpaceBoundaryScroll(ev, SpaceDashboard, a.RoamScroll.Y >= a.RoamScroll.MaxY-1)
		}).
		Children(func() {
		body := func() {
			ui.Column(c).FillWidth().Padding(10, padX, 88, padX).Gap(22).Children(func() {
				a.renderPosterStudioSection(c, pal)
				a.renderRoamSlicesSection(c, pal, phone)
			})
		}
		if phone {
			body()
			return
		}
		ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Box(c).FillWidth().MaxWidth(900).Children(body)
		})
	})
}

func (a *App) renderPosterStudioSection(c *ui.Context, pal SurfacePalette) {
	targetNode, evCount := a.resolveShareTarget()

	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "分享卡片").FontSize(18).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0)
			if PillAction(c, "roam_import_capsule", IconUpload, "导入备份", false).Clicked() {
				mygo.Haptics.Light()
				a.importCapsuleAsync()
			}
		})

		if targetNode.ID == "" {
			ui.Column(c).FillWidth().Padding(24).Center().Gap(8).Radius(16).
				Background(pal.Card).Border(1, pal.Border).
				Children(func() {
					ui.Text(c, "还没有可分享的灵感").FontSize(14).Bold().TextColor(pal.Ink)
					ui.Text(c, "先记下一条灵感，就能生成分享海报了。").
						FontSize(12).TextColor(pal.InkSecondary).TextAlign(ui.Center)
				})
			return
		}

		if len(a.Nodes) > 0 {
			opts := nodeSelectOptions(a.Nodes)
			selectedID := targetNode.ID
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "选一条灵感").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary).Shrink(0)
				if atelierSelect(c, pal, "roam_poster_node_select", "选一条灵感", &selectedID, opts) {
					a.ShareNodeID = selectedID
				}
			})
		}

		// Live Poster Preview on a single clean shadow card (no 4-layer nested frames!)
		ui.Box(c).FillWidth().Radius(18).
			Shadow(0, 4, 18, 0, ui.RGBA(18, 22, 33, 0.06)).
			Children(func() {
				renderPosterArtCard(c, pal, targetNode, evCount, false)
			})

		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if PillAction(c, "roam_export_poster", IconDownload, "导出海报", true).Clicked() {
				a.exportAndMaybeSharePNG(targetNode, false)
			}
			if PillAction(c, "roam_system_share", IconShare, "系统分享", false).Clicked() {
				a.exportAndMaybeSharePNG(targetNode, true)
			}
			if PillAction(c, "roam_export_capsule", IconArchive, "导出备份", false).Clicked() {
				a.exportOrbCapsule(targetNode.ID)
			}
		})

		if a.LastExportedPath != "" {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(6).Radius(10).AlignItems(ui.Center).
				Background(pal.Card).Border(1, pal.Border).
				Children(func() {
					ui.Icon(c, IconCheck).Size(13, 13).TextColor(ColorSuccessInk).Shrink(0)
					ui.Text(c, a.LastExportedPath).FontSize(11).TextColor(pal.InkSecondary).
						MaxLines(2).Grow(1).MinWidth(0)
				})
		}
	})
}

// renderPosterArtCard renders the unified Bento Gallery Emblem poster layout shared across
// live preview, share sheet modal, and 1200x1640 PNG export.
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
		// 1. Header: NEROMANCER
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
			ui.Text(c, "NEROMANCER").
				FontSize(9.5).FontWeight(700).LetterSpacing(1.6).
				TextColor(badgeText).SingleLine().MinWidth(0)
		})

		ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.85))

		if exportMode {
			ui.Spacer(c).Grow(1)
		}

		// 2. Centered Soft-Tinted SVG Emblem + Title + Summary + Tonal Tags + Caption (no outer double frame, no '#'!)
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

		// 3. Footer: Provenance & Date
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Textf(c, "%d 条原记录", evCount).FontSize(10.5).TextColor(pal.InkMuted)
			ui.Text(c, formatPosterDate(node.UpdatedAt)).FontSize(10.5).FontFeatures("tnum").TextColor(pal.InkMuted)
		})
	})
}

func (a *App) renderRoamSlicesSection(c *ui.Context, pal SurfacePalette, phone bool) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "逛逛别人的想法").FontSize(17).Bold().TextColor(pal.Ink)
		})

		if len(a.Nodes) > 0 {
			opts := nodeSelectOptions(a.Nodes)
			targetID := a.CollisionTargetID
			if targetID == "" {
				targetID = a.Nodes[0].ID
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "和我的这条碰撞").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary).Shrink(0)
				if atelierSelect(c, pal, "roam_collision_select", "选择一条自己的灵感", &targetID, opts) {
					a.CollisionTargetID = targetID
				}
			})
		}

		if len(a.RoamSlices) == 0 {
			ui.Column(c).FillWidth().Padding(24).Center().Gap(6).Radius(16).
				Background(pal.Card).Border(1, pal.Border).
				Children(func() {
					ui.Text(c, "暂时没有新的灵感卡片").FontSize(14).Bold().TextColor(pal.Ink)
				})
			return
		}

		if phone {
			ui.Column(c).FillWidth().Gap(12).Children(func() {
				for _, rs := range a.RoamSlices {
					a.renderRoamSliceCard(c, pal, rs, true)
				}
			})
			return
		}

		for rowStart := 0; rowStart < len(a.RoamSlices); rowStart += 2 {
			start := rowStart
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Stretch).Children(func() {
				for col := 0; col < 2; col++ {
					idx := start + col
					if idx >= len(a.RoamSlices) {
						ui.Box(c).Grow(1).Basis(0)
						continue
					}
					a.renderRoamSliceCard(c, pal, a.RoamSlices[idx], false)
				}
			})
		}
	})
}

func (a *App) renderRoamSliceCard(c *ui.Context, pal SurfacePalette, item domain.RoamSlice, phone bool) {
	orbPal := ColorsForTheme(item.Theme)
	_, badgeBg, badgeText := mindOrbSurface(pal, item.Theme)
	iconKey := domain.InferIdeaIconKey(item.Title, item.Snippet, item.DomainTags, "")

	card := ui.Column(c).Key("roam_"+item.ID).Padding(16).Gap(10).Radius(16).
		Background(pal.Card).
		Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.04))
	if phone {
		card.FillWidth()
	} else {
		card.Grow(1).Basis(0)
	}

	card.Children(func() {
		ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, SVGForIconKey(iconKey)).Size(16, 16).TextColor(badgeText).Shrink(0)
			ui.Text(c, item.AuthorAlias).FontSize(12).FontWeight(600).
				TextColor(badgeText).Grow(1).MinWidth(0).SingleLine()
			ui.Box(c).Size(6, 6).Radius(3).Background(orbPal.Mid).Shrink(0)
		})

		ui.Text(c, item.Title).FontSize(15.5).Bold().LineHeight(1.35).TextColor(pal.Ink)

		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Children(func() {
			ui.Box(c).Width(2.5).Radius(2).Background(orbPal.Mid.Alpha(0.55)).Shrink(0)
			ui.Text(c, item.Snippet).FontSize(13).LineHeight(1.58).
				TextColor(pal.InkSecondary).Grow(1).MinWidth(0)
		})

		if len(item.DomainTags) > 0 {
			ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
				for ti, rawTag := range item.DomainTags {
					tag := cleanTagLabel(rawTag)
					if tag == "" {
						continue
					}
					ui.Text(c, tag).Key(fmt.Sprintf("rtag_%s_%d", item.ID, ti)).
						FontSize(11).FontWeight(500).Padding(2.5, 8).Radius(6).
						Background(badgeBg.Alpha(0.78)).TextColor(badgeText)
				}
			})
		}

		if !phone {
			ui.Spacer(c).Grow(1)
		}

		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Row(c).Grow(1).MinWidth(0).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, IconMapPin).Size(12.5, 12.5).TextColor(pal.InkMuted).Shrink(0)
				ui.Text(c, item.SceneHint).FontSize(11).TextColor(pal.InkMuted).
					SingleLine().Grow(1).MinWidth(0)
			})
			if PillAction(c, "col_"+item.ID, IconCollide, "碰撞", true).Clicked() {
				a.triggerRoamCollision(item, phone)
			}
		})
	})
}

func (a *App) triggerRoamCollision(item domain.RoamSlice, isPhone bool) {
	targetID := a.CollisionTargetID
	if targetID == "" && len(a.Nodes) > 0 {
		targetID = a.Nodes[0].ID
	}
	if targetID == "" {
		a.setErrorBanner("先记下一条自己的灵感，再来碰撞吧")
		return
	}
	mygo.Haptics.Medium()
	roamCopy := item
	a.RunAsync("正在把两条想法放在一起碰撞…", func(ctx context.Context) error {
		hybrid, _, err := a.Deps.Incubate.CollideWithRoam(ctx, targetID, roamCopy)
		if err != nil {
			return err
		}
		a.Dispatch(func() {
			a.SelectNode(hybrid.ID)
			a.ActiveSpace = SpaceArchive
			a.ArchiveMode = ArchiveCards
			if isPhone {
				a.DetailSheetOpen = true
			}
			a.setBanner(fmt.Sprintf("碰撞出了新想法：《%s》", hybrid.Title))
		})
		return nil
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
		a.ShareModalOpen = false
	}

	sheetBg := pal.Card
	mobile.Sheet(c, &a.ShareModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.72, 0.92},
		Background: &sheetBg,
		Label:      "分享海报",
	}, func() {
		if !ok {
			return
		}
		evCount := a.EvCounts[node.ID]

		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "分享海报").FontSize(17).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0)
					if GhostIconAction(c, "share_modal_close", IconClose, "关闭", false).Clicked() {
						a.ShareModalOpen = false
					}
				})

				renderPosterArtCard(c, pal, node, evCount, false)

				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
					if PillAction(c, "share_modal_capsule", IconArchive, "导出备份", false).Clicked() {
						a.ShareModalOpen = false
						a.exportOrbCapsule(node.ID)
					}
					if PillAction(c, "share_modal_poster", IconDownload, "导出海报", true).Clicked() {
						a.ShareModalOpen = false
						a.exportAndMaybeSharePNG(node, runtime.GOOS == "android" || runtime.GOOS == "ios")
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
			a.ArchiveMode = ArchiveCards
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

func (a *App) resolveShareTarget() (domain.IdeaNode, int) {
	if n, ok := a.NodeByID[a.ShareNodeID]; ok {
		return n, a.EvCounts[n.ID]
	}
	if len(a.Nodes) > 0 {
		n := a.Nodes[0]
		return n, a.EvCounts[n.ID]
	}
	return domain.IdeaNode{}, 0
}

func nodeSelectOptions(nodes []domain.IdeaNode) []selectOption {
	opts := make([]selectOption, 0, len(nodes))
	for _, n := range nodes {
		opts = append(opts, selectOption{Value: n.ID, Label: n.Title})
	}
	return opts
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

func formatPosterDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}
