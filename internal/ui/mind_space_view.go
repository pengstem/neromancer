package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

func (a *App) renderArchiveView(c *ui.Context, pushDist, pushThreshold float32) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()

	ui.Column(c).Key("space_archive_root").Fill().Background(pal.Canvas).Children(func() {
		if !phone && a.ArchiveMode != ArchiveRoam {
			// Wide Desktop: Master-Detail Split above the unified bottom return bar.
			ui.Row(c).Key("space_archive_split").Grow(1).FillWidth().MinHeight(0).Background(pal.Canvas).Children(func() {
				ui.Box(c).Width(420).FillHeight().Background(pal.Canvas).
					BorderWidth(0, 1, 0, 0).BorderColor(pal.Border).
					Children(func() {
						ui.Column(c).Fill().Children(func() {
							a.renderArchiveHeader(c, pal, false)
							a.renderArchiveTitleAndTabs(c, pal)
							a.renderArchiveBody(c, pal, false)
						})
					})

				ui.Box(c).Grow(1).FillHeight().Background(pal.Card).Children(func() {
					a.renderDetailContent(c)
				})
			})
		} else {
			ui.Column(c).Key("space_archive").Grow(1).FillWidth().MinHeight(0).Background(pal.Canvas).Children(func() {
				if !phone && a.ArchiveMode == ArchiveRoam {
					ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
						ui.Box(c).FillWidth().MaxWidth(900).Children(func() {
							a.renderArchiveHeader(c, pal, false)
						})
					})
				} else {
					a.renderArchiveHeader(c, pal, phone)
				}

				if a.ArchiveMode != ArchiveRoam {
					a.renderArchiveTitleAndTabs(c, pal)
				}
				a.renderArchiveBody(c, pal, phone)
			})
		}

		// Bottom Pull-Up Handle Bar docked at the lower edge of the Archive floor (physically right above Home).
		a.renderArchiveBottomHandle(c, pal, pushDist, pushThreshold, bottomSafe)
	})
}

func (a *App) renderArchiveHeader(c *ui.Context, pal SurfacePalette, phone bool) {
	padX := float32(16)
	if !phone {
		padX = 20
	}

	ui.Row(c).FillWidth().Padding(14, padX, 6, padX).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "灵感档案室").FontSize(19).Bold().TextColor(pal.Ink).SingleLine().Shrink(1).MinWidth(0)

		ui.Spacer(c).Grow(1)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			if a.ArchiveMode != ArchiveRoam {
				if GhostIconAction(c, "archive_search_btn", IconSearch, "搜索档案", a.ArchiveSearchOpen).
					Size(28, 28).Clicked() {
					a.ArchiveSearchOpen = !a.ArchiveSearchOpen
				}
			}

			ui.Row(c).Padding(3).Gap(2).Radius(9).Shrink(0).
				Background(ui.Hex("#E7E9EF")).
				Children(func() {
					a.renderArchiveModeTab(c, pal, "arch_mode_cards", "档案卡", ArchiveCards)
					a.renderArchiveModeTab(c, pal, "arch_mode_outline", "目录", ArchiveOutline)
					a.renderArchiveModeTab(c, pal, "arch_mode_roam", "漫游", ArchiveRoam)
				})
		})
	})
}

func (a *App) renderArchiveBottomHandle(c *ui.Context, pal SurfacePalette, pushDist, pushThreshold, bottomSafe float32) {
	progress := float32(0)
	if pushThreshold > 0 {
		progress = min(float32(1), max(float32(0), pushDist/pushThreshold))
	}
	ready := pushDist >= pushThreshold && pushThreshold > 0

	label := "上拉回到主页看板"
	if ready {
		label = "松手回到主页看板"
	} else if pushDist > 6 {
		label = "继续上拉回到主页"
	}

	fg := pal.InkSecondary
	grabberCol := pal.InkMuted.Alpha(0.32 + 0.35*progress)
	if ready {
		fg = ColorVermilion
		grabberCol = ColorVermilion.Alpha(0.78)
	}

	bar := ui.ButtonBase(c).Key("archive_back_home").
		FillWidth().
		Padding(9, 16, 10+bottomSafe, 16).
		Background(pal.Canvas).
		BorderWidth(1, 0, 0, 0).BorderColor(pal.Border).
		Cursor(ui.CursorPointer).
		Label("回到主页看板")
	mobile.Ink(bar)
	bar.OnClick(func() {
		mygo.Haptics.Selection()
		a.returnToDashboard()
	}).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
				ui.Icon(c, IconChevronUp).Size(14, 14).TextColor(fg).Shrink(0)
				ui.Text(c, label).FontSize(12).FontWeight(600).TextColor(fg).SingleLine()
			})
			ui.Box(c).Width(36 + 12*progress).Height(4).Radius(2).Background(grabberCol)
		})
	})
}

func (a *App) renderArchiveModeTab(c *ui.Context, pal SurfacePalette, key, label string, mode ArchiveViewMode) {
	active := a.ArchiveMode == mode
	bg := ui.Transparent
	fg := pal.InkSecondary
	weight := 500
	if active {
		bg = pal.Card
		fg = pal.Ink
		weight = 600
	}

	tab := ui.ButtonBase(c).Key(key).
		Padding(4, 9).Radius(6).Center().Shrink(0).
		Background(bg).Cursor(ui.CursorPointer).Label(label)
	if active {
		tab.Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.08))
	}
	mobile.Ink(tab)
	tab.OnClick(func() {
		if a.ArchiveMode != mode {
			mygo.Haptics.Selection()
			a.ArchiveMode = mode
		}
	}).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(weight).TextColor(fg).SingleLine()
	})
}

func (a *App) renderArchiveTitleAndTabs(c *ui.Context, pal SurfacePalette) {
	filtered := a.filteredArchiveNodes()

	ui.Column(c).FillWidth().Padding(0, 16, 0, 16).Gap(8).Children(func() {
		ui.Textf(c, "%d 篇 · 落稿自动匹配图标", len(filtered)).
			FontSize(11.5).TextColor(pal.InkMuted).SingleLine()

		if a.ArchiveSearchOpen || strings.TrimSpace(a.SearchQuery) != "" {
			ui.Row(c).FillWidth().Padding(7, 11).Gap(8).AlignItems(ui.Center).
				Radius(10).Background(pal.Card).Border(1, pal.Border).
				Role(ui.RoleSearch).
				Children(func() {
					ui.Icon(c, IconSearch).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
					ui.TextInputBase(c, &a.SearchQuery).
						Placeholder("筛选标题、内容或标签…").
						PlaceholderColor(pal.InkMuted).
						TextColor(pal.Ink).
						FontSize(13).
						Label("筛选灵感").
						Search().
						Grow(1).MinWidth(0)
					if strings.TrimSpace(a.SearchQuery) != "" {
						if GhostIconAction(c, "arch_search_clear", IconClose, "清空筛选", false).
							Size(24, 24).Clicked() {
							a.SearchQuery = ""
						}
					}
				})
		}

		ui.ScrollHorizontal(c).FillWidth().Scrollbars(false).Children(func() {
			ui.Row(c).Gap(16).BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).Children(func() {
				a.renderUnderlineFilterTab(c, pal, "arch_tag_all", fmt.Sprintf("全部 %d", len(a.Nodes)), a.ActiveTag == "", func() {
					a.ActiveTag = ""
				})

				limit := min(len(a.Tags), 12)
				for i := 0; i < limit; i++ {
					tc := a.Tags[i]
					tagName := tc.Tag
					a.renderUnderlineFilterTab(c, pal, "arch_tag_"+tagName, tagName, a.ActiveTag == tagName, func() {
						if a.ActiveTag == tagName {
							a.ActiveTag = ""
						} else {
							a.ActiveTag = tagName
						}
					})
				}
			})
		})
	})
}

func (a *App) renderUnderlineFilterTab(c *ui.Context, pal SurfacePalette, key, label string, active bool, onTap func()) {
	fg := pal.InkMuted
	weight := 500
	if active {
		fg = pal.Ink
		weight = 700
	}

	tab := ui.ButtonBase(c).Key(key).Padding(2, 0, 8, 0).Shrink(0).
		Cursor(ui.CursorPointer).Label(label)
	if active {
		tab.BorderWidth(0, 0, 2, 0).BorderColor(pal.Ink)
	}
	tab.OnClick(func() {
		mygo.Haptics.Selection()
		onTap()
	}).Children(func() {
		ui.Text(c, label).FontSize(12.5).FontWeight(weight).TextColor(fg).SingleLine()
	})
}

func (a *App) renderArchiveBody(c *ui.Context, pal SurfacePalette, phone bool) {
	switch a.ArchiveMode {
	case ArchiveOutline:
		a.renderArchiveOutline(c, pal, phone)
	case ArchiveRoam:
		a.renderRoamShareView(c)
	default:
		a.renderArchiveCards(c, pal, phone)
	}
}

func (a *App) renderArchiveCards(c *ui.Context, pal SurfacePalette, phone bool) {
	filtered := a.filteredArchiveNodes()

	ui.Scroll(c).Key("archive_cards_scroll").TrackScroll(&a.ArchiveScroll).Grow(1).FillWidth().
		HandleInput(func(ev ui.InputEvent) bool {
			// Pushing upward past the bottom of the Archive Vault smoothly pulls the Dashboard back up.
			return a.handleSpaceBoundaryScroll(ev, SpaceDashboard, a.ArchiveScroll.Y >= a.ArchiveScroll.MaxY-1)
		}).
		Children(func() {
			ui.Column(c).FillWidth().Padding(12, 16, 28, 16).Gap(11).Children(func() {
				if len(filtered) == 0 {
					ui.Column(c).FillWidth().Padding(28, 16).Center().Gap(10).Radius(15).
						Background(pal.Card).Border(1, pal.Border).Children(func() {
						ui.Icon(c, IconBulb).Size(22, 22).TextColor(pal.InkMuted)
						ui.Text(c, "还没有匹配的灵感记录").FontSize(14).FontWeight(600).TextColor(pal.InkSecondary)
						if a.ActiveTag != "" || strings.TrimSpace(a.SearchQuery) != "" {
							if PillAction(c, "arch_reset_filter", IconClose, "查看全部", false).Clicked() {
								a.ActiveTag = ""
								a.SearchQuery = ""
							}
						}
					})
					return
				}

				for _, node := range filtered {
					n := node
					orbPal := ColorsForTheme(n.Theme)
					isSel := !phone && n.ID == a.SelectedID
					evCount := max(1, a.EvCounts[n.ID])

					border := pal.Border
					borderW := float32(1)
					if isSel {
						border = orbPal.Mid
						borderW = 1.5
					}

					card := ui.ButtonBase(c).Key("arch_card_"+n.ID).FillWidth().
						Padding(14, 16).Radius(15).
						Background(pal.Card).Border(borderW, border).
						Cursor(ui.CursorPointer).Label(n.Title)
					mobile.Ink(card)

					card.OnClick(func() {
						mygo.Haptics.Selection()
						a.SelectNode(n.ID)
						if phone {
							a.DetailSheetOpen = true
						}
					}).Children(func() {
						ui.Column(c).FillWidth().Gap(9).Children(func() {
							// Header: Auto-matched SVG Icon Badge + Title + Date
							ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
								IdeaIconBadge(c, domain.ResolveIdeaIconKey(n), n.Theme, IconBadgeProps{
									Size:     38,
									IconSize: 19,
									Radius:   10,
								})
								ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
									ui.Text(c, n.Title).FontSize(15).Bold().TextColor(pal.Ink).MaxLines(2)
									ui.Text(c, formatChineseShortDate(n.UpdatedAt)).
										FontSize(11).FontFeatures("tnum").TextColor(pal.InkMuted)
								})
							})

							// Clean 2-line prose excerpt (no manual truncation breaking ellipsis)
							ui.Text(c, n.ConversationalSummary).
								FontSize(13).LineHeight(1.54).TextColor(pal.InkSecondary).MaxLines(2)

							// Quiet borderless metadata footer
							ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
								ui.Row(c).Gap(8).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
									tagLimit := min(len(n.Tags), 3)
									for i := 0; i < tagLimit; i++ {
										ui.Text(c, "#"+n.Tags[i]).FontSize(11).TextColor(pal.InkMuted).SingleLine()
									}
								})
								metaNote := fmt.Sprintf("%d 条记录", evCount)
								if strings.TrimSpace(n.ActionPlanMD) != "" {
									metaNote = fmt.Sprintf("含行动方案 · %d 条记录", evCount)
								}
								ui.Text(c, metaNote).FontSize(11).TextColor(pal.InkMuted).SingleLine().Shrink(0)
							})
						})
					})
				}
			})
		})
}

func (a *App) renderArchiveOutline(c *ui.Context, pal SurfacePalette, phone bool) {
	a.OutlineChoice = a.SelectedID
	if len(a.Roots) == 0 {
		ui.Column(c).Grow(1).FillWidth().Center().Padding(24, 24, 28, 24).Gap(6).Children(func() {
			ui.Icon(c, IconOutline).FontSize(26).TextColor(pal.InkMuted)
			ui.Text(c, "还没有灵感记录").FontSize(14).FontWeight(600).TextColor(pal.InkSecondary)
		})
		return
	}

	a.OutlineState.Prefix = outlineTreePrefix(pal.InkMuted)

	el := ui.Outline(c, &a.OutlineState, a.Roots, func(id string) []string {
		return a.Children[id]
	}, func(id string) {
		n, ok := a.NodeByID[id]
		if !ok {
			return
		}
		orbPal := ColorsForTheme(n.Theme)
		selTint, _, _ := mindOrbSurface(pal, n.Theme)
		evCount := max(1, a.EvCounts[n.ID])
		isSel := n.ID == a.SelectedID

		row := ui.ButtonBase(c).Key("out_"+n.ID).Grow(1).MinWidth(0).
			Padding(12, 10).Cursor(ui.CursorPointer)
		if isSel {
			row.Radius(12).Background(selTint)
		} else {
			row.Background(ui.Transparent).BorderWidth(0, 0, 1, 0).BorderColor(pal.Border.Alpha(0.7))
		}
		mobile.Ink(row)
		row.OnClick(func() {
			mygo.Haptics.Selection()
			a.SelectNode(n.ID)
			if phone {
				a.DetailSheetOpen = true
			}
		}).Children(func() {
			ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Center).Children(func() {
				IdeaIconBadge(c, domain.ResolveIdeaIconKey(n), n.Theme, IconBadgeProps{
					Size:     30,
					IconSize: 15,
					Radius:   8,
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Text(c, n.Title).FontSize(14.5).FontWeight(600).TextColor(pal.Ink).MaxLines(2)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Textf(c, "%d 条原记录", evCount).FontSize(11.5).TextColor(pal.InkMuted)
						ui.Text(c, "·").FontSize(11.5).TextColor(pal.InkMuted)
						ui.Text(c, formatChineseShortDate(n.UpdatedAt)).
							FontSize(11.5).FontFeatures("tnum").TextColor(pal.InkMuted)
					})
				})
				chevronCol := pal.InkMuted
				if isSel {
					chevronCol = orbPal.BadgeText
				}
				ui.Icon(c, IconChevronRight).FontSize(14).TextColor(chevronCol).Shrink(0)
			})
		})
	}).Padding(8, 16, 28, 16).Gap(2).Grow(1).FillWidth()

	if el.Changed() {
		if row := a.OutlineState.List.Selected; row != nil && *row >= 0 && *row < a.OutlineState.Rows() {
			a.SelectNode(a.OutlineState.Item(*row))
		}
	}
}

func outlineTreePrefix(inkMuted ui.Color) func(ui.OutlineRow) {
	return func(r ui.OutlineRow) {
		if r.Depth == 0 && !r.Branch {
			return
		}
		w := float32(r.Depth)*18 + 18
		arrow := r.Arrow().Size(w, 24)
		if r.Branch {
			open := r.Open
			arrow.Draw(func(p *ui.Painter, rect ui.Rect) {
				cx := rect.X + rect.W - 9
				cy := rect.Y + rect.H/2
				var path ui.Path
				if open {
					path.MoveTo(cx-4, cy-2).LineTo(cx, cy+2.5).LineTo(cx+4, cy-2)
				} else {
					path.MoveTo(cx-2, cy-4).LineTo(cx+2.5, cy).LineTo(cx-2, cy+4)
				}
				p.StrokePath(&path, 1.6, inkMuted)
			})
		}
	}
}

func (a *App) filteredArchiveNodes() []domain.IdeaNode {
	q := strings.ToLower(strings.TrimSpace(a.SearchQuery))
	if a.ActiveTag == "" && q == "" {
		return a.Nodes
	}
	var out []domain.IdeaNode
	for _, n := range a.Nodes {
		if a.ActiveTag != "" {
			hasTag := false
			for _, t := range n.Tags {
				if t == a.ActiveTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(n.Title + " " + n.ConversationalSummary + " " + strings.Join(n.Tags, " "))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, n)
	}
	return out
}

func mindOrbSurface(pal SurfacePalette, theme domain.OrbTheme) (tintBg, badgeBg, badgeText ui.Color) {
	orbPal := ColorsForTheme(theme)
	tintBg = orbPal.CardTint
	badgeBg = orbPal.BadgeBg
	badgeText = orbPal.BadgeText
	return tintBg, badgeBg, badgeText
}

func formatChineseShortDate(t time.Time) string {
	if t.IsZero() {
		return "刚刚"
	}
	return fmt.Sprintf("%d月%d日", t.Month(), t.Day())
}
