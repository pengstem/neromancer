package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

func (a *App) renderArchiveView(c *ui.Context, pushDist, pushThreshold float32) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()
	isGraph := a.ArchiveMode == ArchiveGraph

	ui.Column(c).Key("space_archive_root").Fill().Background(pal.Canvas).Children(func() {
		if !phone {
			// Wide Desktop: Master-Detail Split above the unified bottom return bar.
			leftW := float32(420)
			if isGraph {
				leftW = 440
			}
			ui.Row(c).Key("space_archive_split").Grow(1).FillWidth().MinHeight(0).Background(pal.Canvas).Children(func() {
				ui.Box(c).Width(leftW).FillHeight().Background(pal.Canvas).
					BorderWidth(0, 1, 0, 0).BorderColor(pal.Border.Alpha(0.7)).
					Children(func() {
						ui.Column(c).Fill().Children(func() {
							a.renderArchiveHeader(c, pal, false)
							if !isGraph {
								a.renderArchiveTitleAndTabs(c, pal, false)
							}
							a.renderArchiveBody(c, pal, false)
						})
					})

				ui.Box(c).Grow(1).FillHeight().Background(pal.Card).Children(func() {
					a.renderDetailContent(c)
				})
			})
		} else {
			ui.Column(c).Key("space_archive").Grow(1).FillWidth().MinHeight(0).Background(pal.Canvas).Children(func() {
				a.renderArchiveHeader(c, pal, true)
				if !isGraph {
					a.renderArchiveTitleAndTabs(c, pal, true)
				}
				a.renderArchiveBody(c, pal, true)
			})
		}

		// Bottom Pull-Up Handle Bar docked at the lower edge of the Mind Palace floor.
		a.renderArchiveBottomHandle(c, pal, pushDist, pushThreshold, bottomSafe)
	})
}

func (a *App) renderArchiveHeader(c *ui.Context, pal SurfacePalette, phone bool) {
	padX := float32(16)
	if !phone {
		padX = 18
	}

	ui.Row(c).FillWidth().Padding(13, padX, 9, padX).Gap(8).AlignItems(ui.Center).Background(pal.Canvas).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Baseline).Shrink(1).MinWidth(0).Children(func() {
			ui.Text(c, "思维宫殿").FontSize(19).Bold().TextColor(pal.Ink).SingleLine()
			ui.Textf(c, "%d", len(a.Nodes)).FontSize(12).FontWeight(600).
				FontFeatures("tnum").TextColor(pal.InkMuted).SingleLine()
		})

		ui.Spacer(c).Grow(1)

		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			searchBtn := ui.ButtonBase(c).Key("archive_search_btn").
				Size(30, 30).Radius(8).Center().
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label("搜索宫殿")
			mobile.Ink(searchBtn)
			searchBtn.OnClick(func() {
				mygo.Haptics.Selection()
				a.ArchiveSearchOpen = !a.ArchiveSearchOpen
			}).Children(func() {
				icCol := pal.InkMuted
				if a.ArchiveSearchOpen {
					icCol = pal.Ink
				}
				ui.Icon(c, IconSearch).Size(15.5, 15.5).TextColor(icCol)
			})

			importBtn := ui.ButtonBase(c).Key("archive_import_btn").
				Size(30, 30).Radius(8).Center().
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label("导入备份")
			mobile.Ink(importBtn)
			importBtn.OnClick(func() {
				mygo.Haptics.Light()
				a.importCapsuleAsync()
			}).Children(func() {
				ui.Icon(c, IconUpload).Size(15, 15).TextColor(pal.InkMuted)
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

	fg := pal.InkMuted
	grabberCol := pal.InkMuted.Alpha(0.30 + 0.35*progress)
	if ready {
		fg = ColorVermilion
		grabberCol = ColorVermilion.Alpha(0.78)
	}

	bar := ui.ButtonBase(c).Key("archive_back_home").
		FillWidth().
		Padding(8, 16, 10+bottomSafe, 16).
		Background(pal.Canvas).
		Cursor(ui.CursorPointer).
		Label("回到主页看板")
	mobile.Ink(bar)
	bar.OnClick(func() {
		mygo.Haptics.Selection()
		a.returnToDashboard()
	}).Children(func() {
		ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
				ui.Icon(c, IconChevronUp).Size(13.5, 13.5).TextColor(fg).Shrink(0)
				ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(fg).SingleLine()
			})
			ui.Box(c).Width(36 + 12*progress).Height(3.5).Radius(2).Background(grabberCol)
		})
	})
}

func (a *App) renderArchiveTitleAndTabs(c *ui.Context, pal SurfacePalette, phone bool) {
	winW, _ := c.Size()
	padX := float32(16)
	if !phone {
		padX = 20
	}

	q := strings.TrimSpace(a.SearchQuery)
	showSearch := a.ArchiveSearchOpen || q != ""

	ui.Column(c).FillWidth().Padding(2, padX, 8, padX).Gap(8).Children(func() {
		if showSearch {
			ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
				Radius(12).Background(pal.Card).
				Shadow(0, 2, 8, 0, ui.RGBA(18, 22, 33, 0.04)).
				Role(ui.RoleSearch).
				Children(func() {
					ui.Icon(c, IconSearch).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
					ui.TextInputBase(c, &a.SearchQuery).
						Placeholder("搜索想法、内容或主题…").
						PlaceholderColor(pal.InkMuted).
						TextColor(pal.Ink).
						FontSize(13).
						Label("搜索想法").
						Search().
						Grow(1).MinWidth(0)
					if q != "" {
						if GhostIconAction(c, "arch_search_clear", IconClose, "清空搜索", false).
							Size(24, 24).Clicked() {
							a.SearchQuery = ""
						}
					}
				})
		}

		maxInline := 3
		if phone && winW > 0 && winW < 385 {
			maxInline = 2
		}
		inlineTags := a.buildInlineFilterTags(maxInline)
		overflowCount := max(0, len(a.Tags)-len(inlineTags))

		ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
			a.renderTopFilterCapsule(c, pal, "arch_tag_all", "全部", a.ActiveTag == "", func() {
				a.ActiveTag = ""
				a.ArchiveTagDrawerOpen = false
			})

			for _, tagName := range inlineTags {
				tName := tagName
				isActive := cleanTagLabel(a.ActiveTag) == tName
				a.renderTopFilterCapsule(c, pal, "arch_tag_"+tName, tName, isActive, func() {
					if isActive {
						a.ActiveTag = ""
					} else {
						a.ActiveTag = tName
					}
					a.ArchiveTagDrawerOpen = false
				})
			}

			ui.Spacer(c).Grow(1)

			if overflowCount > 0 || a.ArchiveTagDrawerOpen {
				moreFg := pal.InkSecondary
				moreBg := ui.Transparent
				if a.ArchiveTagDrawerOpen {
					moreFg = pal.Ink
					moreBg = pal.Card
				}
				moreBtn := ui.ButtonBase(c).Key("arch_more_tags_btn").
					Padding(5, 8).Radius(999).Gap(4).AlignItems(ui.Center).Shrink(0).
					Background(moreBg).Cursor(ui.CursorPointer).
					Label("展开全部主题")
				mobile.Ink(moreBtn)
				moreBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.ArchiveTagDrawerOpen = !a.ArchiveTagDrawerOpen
				}).Children(func() {
					ui.Icon(c, IconSliders).Size(12.5, 12.5).TextColor(moreFg)
					if overflowCount > 0 {
						ui.Textf(c, "+%d", overflowCount).FontSize(11.5).FontWeight(600).
							FontFeatures("tnum").TextColor(moreFg).SingleLine()
					} else {
						ui.Text(c, "收起").FontSize(11.5).FontWeight(600).TextColor(moreFg).SingleLine()
					}
				})
			}
		})

		matchedTags := a.matchingDrawerTags(q)
		if a.ArchiveTagDrawerOpen || (q != "" && len(matchedTags) > 0) {
			a.renderArchiveTagDrawer(c, pal, matchedTags, q != "" && !a.ArchiveTagDrawerOpen)
		}
	})
}

func (a *App) buildInlineFilterTags(maxInline int) []string {
	if maxInline <= 0 || len(a.Tags) == 0 {
		if act := cleanTagLabel(a.ActiveTag); act != "" {
			return []string{act}
		}
		return nil
	}

	limit := min(len(a.Tags), maxInline)
	out := make([]string, 0, limit)
	totalRunes := 0
	for i := 0; i < len(a.Tags) && len(out) < limit; i++ {
		t := cleanTagLabel(a.Tags[i].Tag)
		if t == "" {
			continue
		}
		rc := utf8.RuneCountInString(t)
		if len(out) >= 2 && totalRunes+rc > 13 {
			break
		}
		out = append(out, t)
		totalRunes += rc
	}

	act := cleanTagLabel(a.ActiveTag)
	if act == "" {
		return out
	}
	for _, existing := range out {
		if existing == act {
			return out
		}
	}
	if len(out) < maxInline {
		out = append(out, act)
	} else if len(out) > 0 {
		out[len(out)-1] = act
	} else {
		out = []string{act}
	}
	return out
}

func (a *App) matchingDrawerTags(query string) []service.TagCount {
	qLower := strings.ToLower(strings.TrimSpace(query))
	var out []service.TagCount
	for _, tc := range a.Tags {
		clean := cleanTagLabel(tc.Tag)
		if clean == "" {
			continue
		}
		if qLower != "" && !strings.Contains(strings.ToLower(clean), qLower) {
			continue
		}
		out = append(out, service.TagCount{Tag: clean, Count: tc.Count})
	}
	return out
}

func (a *App) renderArchiveTagDrawer(c *ui.Context, pal SurfacePalette, tags []service.TagCount, searchSuggestOnly bool) {
	if searchSuggestOnly && len(tags) > 10 {
		tags = tags[:10]
	}
	ui.Column(c).Key("arch_tag_drawer").FillWidth().Padding(11, 13).Gap(8).Radius(14).
		Background(pal.Card).
		Shadow(0, 3, 12, 0, ui.RGBA(18, 22, 33, 0.05)).
		Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				headerText := fmt.Sprintf("全部主题 · 共 %d 个", len(a.Tags))
				if searchSuggestOnly {
					headerText = "匹配的主题"
				}
				ui.Text(c, headerText).FontSize(11).FontWeight(600).TextColor(pal.InkMuted)
				if a.ArchiveTagDrawerOpen {
					if GhostTextAction(c, "arch_close_tag_drawer", IconChevronUp, "收起主题").Clicked() {
						a.ArchiveTagDrawerOpen = false
					}
				}
			})

			if len(tags) == 0 {
				ui.Text(c, "没有匹配的主题词").FontSize(12).TextColor(pal.InkMuted)
				return
			}

			ui.Scroll(c).FillWidth().MaxHeight(184).Children(func() {
				ui.Row(c).FillWidth().Wrap().Gap(6).Children(func() {
					for _, tc := range tags {
						tName := tc.Tag
						tCount := tc.Count
						active := cleanTagLabel(a.ActiveTag) == tName
						bg := pal.SoftPanel
						fg := pal.Ink
						cntCol := pal.InkMuted
						if active {
							bg = pal.Ink
							fg = pal.Card
							cntCol = pal.Card.Alpha(0.72)
						}

						chip := ui.ButtonBase(c).Key("drawer_tag_" + tName).
							Padding(4.5, 10).Radius(999).Gap(4).AlignItems(ui.Center).
							Background(bg).Cursor(ui.CursorPointer).Label(tName)
						mobile.Ink(chip)
						chip.OnClick(func() {
							mygo.Haptics.Selection()
							if active {
								a.ActiveTag = ""
							} else {
								a.ActiveTag = tName
							}
							a.ArchiveTagDrawerOpen = false
						}).Children(func() {
							ui.Text(c, tName).FontSize(11.5).FontWeight(500).TextColor(fg).SingleLine()
							ui.Textf(c, "%d", tCount).FontSize(10).FontFeatures("tnum").TextColor(cntCol).SingleLine()
						})
					}
				})
			})
		})
}

func (a *App) renderTopFilterCapsule(c *ui.Context, pal SurfacePalette, key, label string, active bool, onTap func()) {
	bg := pal.Card
	fg := pal.InkSecondary
	weight := 500
	if active {
		bg = pal.Ink
		fg = pal.Card
		weight = 600
	}

	pill := ui.ButtonBase(c).Key(key).
		Padding(5, 11).Radius(999).Center().Shrink(0).
		Background(bg).Cursor(ui.CursorPointer).Label(label)
	if !active {
		pill.Shadow(0, 1, 4, 0, ui.RGBA(18, 22, 33, 0.03))
	}
	mobile.Ink(pill)
	pill.OnClick(func() {
		mygo.Haptics.Selection()
		onTap()
	}).Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(weight).TextColor(fg).SingleLine()
	})
}

func (a *App) renderArchiveBody(c *ui.Context, pal SurfacePalette, phone bool) {
	switch a.ArchiveMode {
	case ArchiveGraph:
		a.renderArchivePulseSplit(c, phone)
	default:
		a.renderArchiveCards(c, pal, phone)
	}
}

func (a *App) renderArchiveCards(c *ui.Context, pal SurfacePalette, phone bool) {
	filtered := a.filteredArchiveNodes()

	ui.Scroll(c).Key("archive_cards_scroll").TrackScroll(&a.ArchiveScroll).Grow(1).FillWidth().
		HandleInput(func(ev ui.InputEvent) bool {
			return a.handleSpaceBoundaryScroll(ev, SpaceDashboard, a.ArchiveScroll.Y >= a.ArchiveScroll.MaxY-1)
		}).
		Children(func() {
			ui.Column(c).FillWidth().Padding(6, 16, 68, 16).Gap(10).Children(func() {
				if len(filtered) == 0 {
					ui.Column(c).FillWidth().Padding(28, 16).Center().Gap(10).Radius(16).
						Background(pal.Card).
						Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.04)).
						Children(func() {
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

					cardBg := pal.Card
					if isSel {
						cardBg = orbPal.CardTint
					}

					card := ui.ButtonBase(c).Key("arch_card_"+n.ID).FillWidth().
						Padding(15, 16).Radius(16).
						Background(cardBg).
						Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.04)).
						Cursor(ui.CursorPointer).Label(n.Title)
					if isSel {
						card.Border(1.5, orbPal.Mid.Alpha(0.45))
					}
					mobile.Ink(card)

					card.OnClick(func() {
						mygo.Haptics.Selection()
						a.SelectNode(n.ID)
						if phone {
							a.DetailSheetOpen = true
						}
					}).Children(func() {
						ui.Column(c).FillWidth().Gap(8).Children(func() {
							// Header: Naked auto-matched semantic SVG Icon + Title
							ui.Row(c).FillWidth().Gap(8.5).AlignItems(ui.Start).Children(func() {
								ui.Box(c).Padding(2.5, 0, 0, 0).Shrink(0).Children(func() {
									ui.Icon(c, SVGForIconKey(domain.ResolveIdeaIconKey(n))).
										Size(17.5, 17.5).TextColor(orbPal.BadgeText)
								})
								ui.Text(c, n.Title).FontSize(15).Bold().LineHeight(1.34).
									TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(2)
							})

							// Clean 2-line spoken prose excerpt
							ui.Text(c, n.ConversationalSummary).
								FontSize(13).LineHeight(1.54).TextColor(pal.InkSecondary).MaxLines(2)

							// Footer: Tonal soft-tint tag chips WITHOUT '#' + Tabular date/evidence count
							ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Padding(2, 0, 0, 0).Children(func() {
								ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).MinWidth(0).Wrap().Children(func() {
									shown := 0
									for _, rawTag := range n.Tags {
										tg := cleanTagLabel(rawTag)
										if tg == "" {
											continue
										}
										ui.Box(c).Padding(2.5, 8).Radius(6).
											Background(orbPal.BadgeBg.Alpha(0.78)).
											Children(func() {
												ui.Text(c, tg).FontSize(11).FontWeight(500).
													TextColor(orbPal.BadgeText).SingleLine()
											})
										shown++
										if shown >= 3 {
											break
										}
									}
								})

								metaNote := fmt.Sprintf("%s · %d 条", formatChineseShortDate(n.UpdatedAt), evCount)
								if strings.TrimSpace(n.ActionPlanMD) != "" {
									metaNote = fmt.Sprintf("含方案 · %s · %d 条", formatChineseShortDate(n.UpdatedAt), evCount)
								}
								ui.Text(c, metaNote).FontSize(11).FontFeatures("tnum").
									TextColor(pal.InkMuted).SingleLine().Shrink(0)
							})
						})
					})
				}
			})
		})
}

func (a *App) filteredArchiveNodes() []domain.IdeaNode {
	q := strings.ToLower(strings.TrimSpace(a.SearchQuery))
	activeTag := cleanTagLabel(a.ActiveTag)
	if activeTag == "" && q == "" {
		return a.Nodes
	}
	var out []domain.IdeaNode
	for _, n := range a.Nodes {
		if activeTag != "" {
			hasTag := false
			for _, t := range n.Tags {
				if cleanTagLabel(t) == activeTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(n.Title + " " + n.Folder + " " + n.Visual.Caption + " " + n.ConversationalSummary + " " + strings.Join(n.Tags, " "))
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
