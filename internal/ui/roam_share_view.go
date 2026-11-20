package ui

import (
	"context"
	"fmt"
	"image"
	"math"
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
	phase := a.Phase(c)
	phone := a.isPhoneLayout(c)

	padX := float32(16)
	if !phone {
		padX = 28
	}

	ui.Scroll(c).Key("space_roam").Fill().Background(pal.Canvas).Children(func() {
		ui.Column(c).FillWidth().Padding(14, padX, 114, padX).Gap(24).Children(func() {
			a.renderPosterStudioSection(c, pal, phase)
			a.renderRoamSlicesSection(c, pal, phase, phone)
		})
	})
}

func (a *App) renderPosterStudioSection(c *ui.Context, pal SurfacePalette, phase float32) {
	targetNode, evCount := a.resolveShareTarget()

	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "记忆胶囊与海报").FontSize(18).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0)
			if PillAction(c, "roam_import_capsule", IconUpload, "导入胶囊", false).Clicked() {
				mygo.Haptics.Light()
				a.importCapsuleAsync()
			}
		})

		if targetNode.ID == "" {
			ui.Column(c).FillWidth().Padding(24).Center().Gap(8).Radius(16).
				Background(pal.Card).Border(1, pal.Border).
				Children(func() {
					ui.Text(c, "暂无可展出的记忆球").FontSize(14).Bold().TextColor(pal.Ink)
					ui.Text(c, "记下一条灵感或导入他人分享的胶囊即可生成海报。").
						FontSize(12).TextColor(pal.InkSecondary).TextAlign(ui.Center)
				})
			return
		}

		if len(a.Nodes) > 0 {
			opts := nodeSelectOptions(a.Nodes)
			selectedID := targetNode.ID
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "选择记忆球").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary).Shrink(0)
				if atelierSelect(c, pal, "roam_poster_node_select", "选择记忆球", &selectedID, opts) {
					a.ShareNodeID = selectedID
				}
			})
		}

		orbPal := ColorsForTheme(targetNode.Theme)
		softTint, badgeBg, badgeText := mindOrbSurface(pal, a.DarkMode, targetNode.Theme)

		// Framed Live Poster Preview card
		ui.Column(c).FillWidth().Padding(14).Radius(18).
			Background(pal.Card).Border(1, pal.Border).
			Shadow(0, 4, 18, 0, ui.RGBA(0, 0, 0, 0.07)).
			Children(func() {
				ui.Column(c).FillWidth().Padding(16).Gap(12).Radius(12).
					Border(1, pal.Border.Alpha(0.85)).
					LinearGradient(ui.LinearGradient{
						From:  pal.Card,
						To:    softTint,
						Angle: 180,
						Oklab: true,
					}).
					Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Gap(8).Children(func() {
							ui.Text(c, "NEROMANCER  /  MEMORY ORB").
								FontSize(9.5).FontWeight(600).LetterSpacing(1.4).
								TextColor(badgeText).SingleLine().MinWidth(0)
							ui.Text(c, targetNode.Stage.Label()).
								FontSize(10).FontWeight(600).Padding(2, 8).Radius(999).
								Background(badgeBg).TextColor(badgeText).Shrink(0)
						})

						ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.75))

						ui.Box(c).Size(104, 104).AlignSelf(ui.Center).Draw(func(p *ui.Painter, r ui.Rect) {
							drawPosterOrbFrame(p, r, targetNode, phase, true, pal.Border, orbPal.Mid.Alpha(0.32), 84)
						})

						ui.Text(c, targetNode.Title).
							FontSize(17).Bold().LineHeight(1.32).
							TextColor(pal.Ink).TextAlign(ui.Center).Balance()

						ui.Text(c, "“"+trimText(targetNode.ConversationalSummary, 76)+"”").
							FontSize(13).LineHeight(1.62).
							TextColor(pal.InkSecondary).TextAlign(ui.Center).MaxLines(3)

						if capText := strings.TrimSpace(targetNode.Visual.Caption); capText != "" {
							ui.Text(c, capText).FontSize(11).Italic().TextColor(pal.InkMuted).TextAlign(ui.Center)
						}

						if len(targetNode.Tags) > 0 {
							ui.Row(c).FillWidth().Wrap().Gap(6).Center().Children(func() {
								limit := min(len(targetNode.Tags), 4)
								for i := 0; i < limit; i++ {
									tag := targetNode.Tags[i]
									ui.Text(c, "#"+tag).Key(fmt.Sprintf("prev_tag_%s_%d", targetNode.ID, i)).
										FontSize(10.5).FontWeight(500).Padding(2, 8).Radius(999).
										Background(badgeBg).TextColor(badgeText)
								}
							})
						}

						ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.75))

						dateStr := formatPosterDate(targetNode.UpdatedAt)
						ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Textf(c, "%d 份原始证据", evCount).FontSize(10.5).TextColor(pal.InkMuted)
							ui.Text(c, dateStr).FontSize(10.5).FontFeatures("tnum").TextColor(pal.InkMuted)
						})
					})
			})

		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if PillAction(c, "roam_export_poster", IconDownload, "导出海报", true).Clicked() {
				a.exportAndMaybeSharePNG(targetNode, false)
			}
			if PillAction(c, "roam_system_share", IconShare, "系统分享", false).Clicked() {
				a.exportAndMaybeSharePNG(targetNode, true)
			}
			if PillAction(c, "roam_export_capsule", IconArchive, "导出胶囊", false).Clicked() {
				a.exportOrbCapsule(targetNode.ID)
			}
		})

		if a.LastExportedPath != "" {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(6).Radius(10).AlignItems(ui.Center).
				Background(pal.SoftPanel).Border(1, pal.Border.Alpha(0.7)).
				Children(func() {
					ui.Icon(c, IconCheck).Size(13, 13).TextColor(c.Theme().Success).Shrink(0)
					ui.Text(c, a.LastExportedPath).FontSize(11).TextColor(pal.InkSecondary).
						MaxLines(2).Grow(1).MinWidth(0)
				})
		}
	})
}

func (a *App) renderRoamSlicesSection(c *ui.Context, pal SurfacePalette, phase float32, phone bool) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "他人灵感切片").FontSize(17).Bold().TextColor(pal.Ink)
			ui.Text(c, "只读").FontSize(10.5).FontWeight(600).Padding(2, 8).Radius(999).
				Background(pal.SoftPanel).TextColor(pal.InkSecondary).Shrink(0)
		})

		if len(a.Nodes) > 0 {
			opts := nodeSelectOptions(a.Nodes)
			targetID := a.CollisionTargetID
			if targetID == "" {
				targetID = a.Nodes[0].ID
			}
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "碰撞对象").FontSize(12).FontWeight(600).TextColor(pal.InkSecondary).Shrink(0)
				if atelierSelect(c, pal, "roam_collision_select", "选择碰撞对象", &targetID, opts) {
					a.CollisionTargetID = targetID
				}
			})
		}

		if len(a.RoamSlices) == 0 {
			ui.Column(c).FillWidth().Padding(24).Center().Gap(6).Radius(16).
				Background(pal.Card).Border(1, pal.Border).
				Children(func() {
					ui.Text(c, "暂无可漫游切片").FontSize(14).Bold().TextColor(pal.Ink)
				})
			return
		}

		ui.Row(c).FillWidth().Wrap().Gap(12).Children(func() {
			for idx, rs := range a.RoamSlices {
				item := rs
				orbPal := ColorsForTheme(item.Theme)
				_, badgeBg, badgeText := mindOrbSurface(pal, a.DarkMode, item.Theme)

				card := ui.Column(c).Key("roam_"+item.ID).Padding(16).Gap(10).Radius(16).
					Background(pal.Card).Border(1, pal.Border)
				if phone {
					card.FillWidth()
				} else {
					card.Width(336)
				}

				card.Children(func() {
					ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(24, 24).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
							DrawAnimatedMemoryOrb(p, r, item.Theme, 0.82, phase+float32(idx)*0.65, 0.35, "orbit", false)
						})
						ui.Text(c, item.AuthorAlias).FontSize(12).FontWeight(600).
							TextColor(badgeText).Grow(1).MinWidth(0).SingleLine()
						ui.Box(c).Size(6, 6).Radius(3).Background(orbPal.Rim).Shrink(0)
					})

					ui.Text(c, item.Title).FontSize(15.5).Bold().LineHeight(1.35).TextColor(pal.Ink)

					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Children(func() {
						ui.Box(c).Width(2.5).Radius(2).Background(orbPal.Rim.Alpha(0.55)).Shrink(0)
						ui.Text(c, item.Snippet).FontSize(13).LineHeight(1.58).
							TextColor(pal.InkSecondary).Grow(1).MinWidth(0)
					})

					if len(item.DomainTags) > 0 {
						ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
							for ti, tag := range item.DomainTags {
								ui.Text(c, "#"+tag).Key(fmt.Sprintf("rtag_%s_%d", item.ID, ti)).
									FontSize(11).FontWeight(500).Padding(3, 9).Radius(999).
									Background(badgeBg).TextColor(badgeText)
							}
						})
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
		})
	})
}

func (a *App) triggerRoamCollision(item domain.RoamSlice, isPhone bool) {
	targetID := a.CollisionTargetID
	if targetID == "" && len(a.Nodes) > 0 {
		targetID = a.Nodes[0].ID
	}
	if targetID == "" {
		a.setErrorBanner("先记下一颗自己的记忆球再发起碰撞")
		return
	}
	mygo.Haptics.Medium()
	roamCopy := item
	a.RunAsync("正在将两条灵感碰撞融合…", func(ctx context.Context) error {
		hybrid, _, err := a.Deps.Incubate.CollideWithRoam(ctx, targetID, roamCopy)
		if err != nil {
			return err
		}
		a.Dispatch(func() {
			a.SelectNode(hybrid.ID)
			if isPhone {
				a.DetailSheetOpen = true
			} else {
				a.ActiveSpace = SpaceMind
			}
			a.setBanner(fmt.Sprintf("碰撞生成新记忆球：《%s》", hybrid.Title))
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
	a.setBanner("已导出便携灵感胶囊（含原始证据）：" + p)
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
			Text:  fmt.Sprintf("Neromancer 记忆球：《%s》", node.Title),
			Files: []string{p},
		}); err != nil {
			a.setBanner("海报已保存至本地：" + p)
			return
		}
	}
	a.setBanner("已导出记忆球海报：" + p)
}

func (a *App) renderShareModal(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)

	node, ok := a.NodeByID[a.ShareNodeID]
	if !ok {
		a.ShareModalOpen = false
	}

	sheetBg := pal.Card
	mobile.Sheet(c, &a.ShareModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.68, 0.90},
		Background: &sheetBg,
		Label:      "记忆胶囊与海报",
	}, func() {
		if !ok {
			return
		}
		orbPal := ColorsForTheme(node.Theme)
		softTint, badgeBg, badgeText := mindOrbSurface(pal, a.DarkMode, node.Theme)
		evCount := a.EvCounts[node.ID]

		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "记忆球展签").FontSize(17).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0)
					if PillAction(c, "share_modal_close", IconClose, "关闭", false).Clicked() {
						a.ShareModalOpen = false
					}
				})

				ui.Column(c).FillWidth().Padding(16).Gap(10).Radius(16).
					Border(1, pal.Border).
					LinearGradient(ui.LinearGradient{
						From:  pal.Card,
						To:    softTint,
						Angle: 180,
						Oklab: true,
					}).
					Center().
					Children(func() {
						ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
							ui.Text(c, "NEROMANCER  /  MEMORY ORB").
								FontSize(9.5).FontWeight(600).LetterSpacing(1.2).TextColor(badgeText)
							ui.Text(c, node.Stage.Label()).FontSize(10).FontWeight(600).
								Padding(2, 8).Radius(999).Background(badgeBg).TextColor(badgeText)
						})
						ui.Box(c).Size(98, 98).Draw(func(p *ui.Painter, r ui.Rect) {
							drawPosterOrbFrame(p, r, node, phase, true, pal.Border, orbPal.Mid.Alpha(0.32), 84)
						})
						ui.Text(c, node.Title).FontSize(17).Bold().TextColor(pal.Ink).TextAlign(ui.Center).Balance()
						ui.Text(c, "“"+trimText(node.ConversationalSummary, 84)+"”").
							FontSize(12.5).LineHeight(1.58).TextColor(pal.InkSecondary).TextAlign(ui.Center).MaxLines(3)
						if capText := strings.TrimSpace(node.Visual.Caption); capText != "" {
							ui.Text(c, capText).FontSize(11).Italic().TextColor(pal.InkMuted).TextAlign(ui.Center)
						}
						ui.Textf(c, "%d 份原始证据 · %s", evCount, formatPosterDate(node.UpdatedAt)).
							FontSize(10.5).FontFeatures("tnum").TextColor(pal.InkMuted)
					})

				ui.Row(c).FillWidth().Wrap().Justify(ui.End).Gap(8).Children(func() {
					if PillAction(c, "share_modal_capsule", IconArchive, "导出胶囊", false).Clicked() {
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
			Title:   "导入 .orb.json 灵感胶囊",
			Filters: []mygo.FileFilter{{Name: "Orb Capsule", Extensions: []string{"json"}}},
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
			a.ActiveSpace = SpaceMind
			a.DetailSheetOpen = true
			a.setBanner("已导入灵感胶囊并保留原始证据：《" + node.Title + "》")
		})
	}()
}

// RenderSharePoster renders a 600x820 @ 2x (1200x1640) museum-grade archival PNG exhibition poster.
func RenderSharePoster(node domain.IdeaNode, evCount int) *image.RGBA {
	paperPal := PaletteForMode(false)
	orbPal := ColorsForTheme(node.Theme)
	frameRule := ui.Hex("#D4CBB9")
	bottomWash := ui.Hex("#FCF9F4").MixOklab(orbPal.CardTint, 0.88)

	return ui.Render(func(c *ui.Context) {
		ui.Themed(c, NeromancerThemeFor(false)).Fill().Children(func() {
			ui.Column(c).Fill().Padding(20).
				LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#FCF9F4"),
					To:    bottomWash,
					Angle: 180,
					Oklab: true,
				}).
				Children(func() {
					ui.Column(c).Fill().Border(1, frameRule).Padding(28, 34, 24, 34).
						Gap(14).AlignItems(ui.Center).
						Children(func() {
							ui.Text(c, "NEROMANCER  /  MEMORY ORB").
								FontSize(10.5).FontWeight(600).LetterSpacing(2.2).
								TextColor(orbPal.BadgeText).SingleLine()

							ui.Box(c).Width(42).Height(1).Background(orbPal.Rim.Alpha(0.45))

							ui.Box(c).Size(228, 228).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
								drawPosterOrbFrame(p, r, node, 0.35, false, frameRule, orbPal.Mid.Alpha(0.28), 200)
							})

							ui.Text(c, node.Title).
								FontSize(26).Bold().LineHeight(1.30).
								TextColor(paperPal.Ink).TextAlign(ui.Center).Balance()

							ui.Text(c, node.ConversationalSummary).
								FontSize(15).LineHeight(1.65).
								TextColor(paperPal.InkSecondary).TextAlign(ui.Center).Pretty().MaxLines(6)

							if len(node.Tags) > 0 {
								ui.Row(c).Wrap().Gap(8).Center().Children(func() {
									for _, tag := range node.Tags {
										ui.Text(c, "#"+tag).FontSize(11.5).FontWeight(500).
											Padding(4, 11).Radius(999).
											Background(orbPal.BadgeBg).TextColor(orbPal.BadgeText)
									}
								})
							}

							if capText := strings.TrimSpace(node.Visual.Caption); capText != "" {
								ui.Text(c, capText).FontSize(12).Italic().
									TextColor(paperPal.InkMuted).TextAlign(ui.Center)
							}

							ui.Spacer(c).Grow(1)

							ui.Box(c).FillWidth().Height(1).Background(frameRule.Alpha(0.85))

							ui.Textf(c, "%s  ·  %d 份原始证据  ·  %s",
								node.Stage.Label(), evCount, formatPosterDate(node.UpdatedAt)).
								FontSize(11).FontFeatures("tnum").LetterSpacing(0.6).
								TextColor(paperPal.InkMuted).TextAlign(ui.Center)
						})
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
					item.Background(pal.Accent).TextColor(pal.AccentText)
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

func drawPosterOrbFrame(p *ui.Painter, r ui.Rect, node domain.IdeaNode, phase float32, animated bool, outerCol, innerCol ui.Color, orbDim float32) {
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	maxDim := min(r.W, r.H)
	if maxDim <= 8 {
		return
	}
	if orbDim > maxDim {
		orbDim = maxDim * 0.86
	}
	outerR := min(maxDim*0.48, orbDim*0.56)
	innerR := outerR * 0.94

	var outerRing ui.Path
	outerRing.Circle(cx, cy, outerR)
	p.StrokePath(&outerRing, 1.0, outerCol.Alpha(0.85))

	var innerRing ui.Path
	innerRing.Circle(cx, cy, innerR)
	p.StrokePath(&innerRing, 0.8, innerCol)

	for i := 0; i < 4; i++ {
		ang := float64(i) * math.Pi / 2.0
		tx := cx + outerR*float32(math.Cos(ang))
		ty := cy + outerR*float32(math.Sin(ang))
		var dot ui.Path
		dot.Circle(tx, ty, 1.6)
		p.FillPath(&dot, outerCol)
	}

	orbRect := ui.Rect{
		X: cx - orbDim/2,
		Y: cy - orbDim/2,
		W: orbDim,
		H: orbDim,
	}
	if animated {
		DrawAnimatedMemoryOrb(p, orbRect, node.Theme, node.Weight, phase, float32(node.Visual.Energy), node.Visual.MotifType, false)
	} else {
		DrawMemoryOrb(p, orbRect, node.Theme, node.Weight, node.Visual.MotifType, false)
	}
}

func formatPosterDate(t time.Time) string {
	if t.IsZero() {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}
