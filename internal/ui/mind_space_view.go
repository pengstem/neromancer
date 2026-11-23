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
				ui.Box(c).Width(leftW).FillHeight().Clip().Background(pal.Canvas).
					BorderWidth(0, 1, 0, 0).BorderColor(pal.Border.Alpha(0.7)).
					Children(func() {
						ui.Column(c).Fill().MinHeight(0).Children(func() {
							a.renderArchiveHeader(c, pal, false)
							if !isGraph {
								a.renderArchiveTitleAndTabs(c, pal, false)
							}
							a.renderArchiveBody(c, pal, false)
						})
					})

				ui.Box(c).Grow(1).FillHeight().Clip().Background(pal.Card).Children(func() {
					a.renderDetailContent(c)
				})
			})
		} else {
			ui.Column(c).Key("space_archive").Grow(1).FillWidth().MinHeight(0).Clip().Background(pal.Canvas).Children(func() {
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

	portalGlow := a.mindPalacePortalPulse(c)
	countBg := pal.Card
	countBorder := pal.Border.Alpha(0.65)
	countFg := pal.InkSecondary
	if portalGlow > 0.005 {
		countBg = lerpColor(countBg, ColorVermilion.Alpha(0.12), portalGlow)
		countBorder = lerpColor(countBorder, ColorVermilion.Alpha(0.55), portalGlow)
		countFg = lerpColor(countFg, ColorVermilion, portalGlow*0.9)
	}

	ui.Row(c).FillWidth().Padding(12, padX, 8, padX).Gap(8).AlignItems(ui.Center).Background(pal.Canvas).Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
			ui.Text(c, "思维宫殿").FontSize(FontTitle).Bold().TextColor(pal.Ink).SingleLine()
			ui.Textf(c, "%d", len(a.Nodes)).FontSize(FontCaption).FontWeight(600).
				FontFeatures("tnum").Padding(2, 7).Radius(RadiusPill).
				Background(countBg).Border(1, countBorder).
				Scale(1.0+0.08*portalGlow, 1.0+0.08*portalGlow).
				TextColor(countFg).SingleLine()
		})

		ui.Spacer(c).Grow(1)

		ui.Row(c).Gap(6).AlignItems(ui.Center).Shrink(0).Children(func() {
			if GhostIconAction(c, "archive_search_btn", IconSearch, "搜索宫殿", a.ArchiveSearchOpen).
				Tooltip("搜索宫殿").Clicked() {
				mygo.Haptics.Selection()
				a.ArchiveSearchOpen = !a.ArchiveSearchOpen
			}

			if GhostIconAction(c, "archive_import_btn", IconUpload, "导入备份", false).
				Tooltip("导入备份").Clicked() {
				mygo.Haptics.Light()
				a.importCapsuleAsync()
			}

			addBtn := GhostIconAction(c, "archive_quick_add_btn", IconPlus, "随手记一笔", false).
				Bind(&a.ArchiveAddHandle).Tooltip("随手记一笔")
			if addBtn.Clicked() {
				mygo.Haptics.Light()
				a.closeOrbDials()
				a.openCaptureModal(1, a.ArchiveAddHandle.Bounds(c))
			}
		})
	})
}

func (a *App) renderArchiveBottomHandle(c *ui.Context, pal SurfacePalette, pushDist, pushThreshold, bottomSafe float32) {
	progress := float32(0)
	if pushThreshold > 0 {
		progress = min(float32(1), max(float32(0), pushDist/pushThreshold))
	}
	ready := pushDist >= pushThreshold && pushThreshold > 0

	label := ""
	if ready {
		label = "松手回到主页看板"
	} else if pushDist > 6 {
		label = "继续上拉"
	}

	fg := pal.InkMuted
	grabberCol := pal.InkMuted.Alpha(0.30 + 0.35*progress)
	if ready {
		fg = ColorVermilion
		grabberCol = ColorVermilion.Alpha(0.78)
	}

	ui.Row(c).FillWidth().Justify(ui.Center).Padding(0, 16, bottomSafe, 16).Background(pal.Canvas).Children(func() {
		bar := ui.ButtonBase(c).Key("archive_back_home").
			MinWidth(136).
			Padding(8, 24, 10, 24).
			Radius(RadiusPill).
			Background(ui.Transparent).
			Cursor(ui.CursorPointer).
			Label("回到主页看板")
		mobile.Ink(bar)
		bar.OnClick(func() {
			mygo.Haptics.Selection()
			a.returnToDashboard()
		}).Children(func() {
			ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
					ui.Icon(c, IconChevronUp).Size(13.5, 13.5).TextColor(fg).Shrink(0)
					if label != "" {
						ui.Text(c, label).FontSize(11.5).FontWeight(600).TextColor(fg).SingleLine()
					}
				})
				ui.Box(c).Width(36 + 12*progress).Height(3.5).Radius(2).Background(grabberCol)
			})
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
			RenderSearchBar(c, pal, "arch_search", "搜索想法、内容或主题…", "搜索想法", &a.SearchQuery)
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
					Padding(5, 8).Radius(RadiusPill).Gap(4).AlignItems(ui.Center).Shrink(0).
					Background(moreBg).Cursor(ui.CursorPointer).
					Label("展开全部主题")
				if a.ArchiveTagDrawerOpen {
					moreBtn.Border(1, pal.Border.Alpha(0.75))
				}
				mobile.Ink(moreBtn)
				moreBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.ArchiveTagDrawerOpen = !a.ArchiveTagDrawerOpen
				}).Children(func() {
					ui.Icon(c, IconSliders).Size(13, 13).TextColor(moreFg)
					if overflowCount > 0 {
						ui.Textf(c, "+%d", overflowCount).FontSize(FontMeta).FontWeight(600).
							FontFeatures("tnum").TextColor(moreFg).SingleLine()
					} else {
						ui.Text(c, "收起").FontSize(FontMeta).FontWeight(600).TextColor(moreFg).SingleLine()
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
	StyleSurfaceCard(
		ui.Column(c).Key("arch_tag_drawer").FillWidth().Padding(11, 13).Gap(8),
		pal,
	).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			headerText := "全部主题"
			if searchSuggestOnly {
				headerText = "匹配的主题"
			}
			ui.Text(c, headerText).FontSize(FontCaption).FontWeight(600).TextColor(pal.InkMuted)
			if a.ArchiveTagDrawerOpen {
				if GhostTextAction(c, "arch_close_tag_drawer", IconChevronUp, "收起主题").Clicked() {
					a.ArchiveTagDrawerOpen = false
				}
			}
		})

		if len(tags) == 0 {
			ui.Text(c, "没有匹配的主题词").FontSize(FontMeta).TextColor(pal.InkMuted)
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
						Padding(5, 10).Radius(RadiusPill).Gap(4).AlignItems(ui.Center).
						Background(bg).Cursor(ui.CursorPointer).Label(tName)
					if !active {
						chip.Border(1, pal.Border.Alpha(0.65))
					}
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
						ui.Text(c, tName).FontSize(FontMeta).FontWeight(500).TextColor(fg).SingleLine()
						ui.Textf(c, "%d", tCount).FontSize(FontCaption).FontFeatures("tnum").TextColor(cntCol).SingleLine()
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
		Padding(5, 11).Radius(RadiusPill).Center().Shrink(0).
		Background(bg).Cursor(ui.CursorPointer).Label(label)
	if !active {
		pill.Border(1, pal.Border.Alpha(0.65)).Shadow(0, 1, 4, 0, ui.RGBA(18, 22, 33, 0.025))
	}
	mobile.Ink(pill)
	pill.OnClick(func() {
		mygo.Haptics.Selection()
		onTap()
	}).Children(func() {
		ui.Text(c, label).FontSize(FontMeta).FontWeight(weight).TextColor(fg).SingleLine()
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

	ui.Box(c).Key("archive_cards_wrap").Grow(1).FillWidth().MinHeight(0).Children(func() {
		ui.Scroll(c).Key("archive_cards_scroll").TrackScroll(&a.ArchiveScroll).Fill().
			HandleInput(func(ev ui.InputEvent) bool {
				return a.handleSpaceBoundaryScroll(ev, SpaceDashboard, a.ArchiveScroll.Y >= a.ArchiveScroll.MaxY-1)
			}).
			Children(func() {
				ui.Column(c).FillWidth().Padding(6, 16, 86, 16).Gap(10).Children(func() {
					if len(filtered) == 0 {
						StyleSurfaceCard(
							ui.Column(c).FillWidth().Padding(28, 16).Center().Gap(10),
							pal,
						).Children(func() {
							ui.Icon(c, IconBulb).Size(20, 20).TextColor(pal.InkMuted)
							ui.Text(c, "还没有匹配的灵感记录").FontSize(FontBody).FontWeight(600).TextColor(pal.InkSecondary)
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

						card := StyleSurfaceCard(
							ui.ButtonBase(c).Key("arch_card_"+n.ID).FillWidth().Padding(15, 16),
							pal,
						).Cursor(ui.CursorPointer).Label(n.Title)
						if isSel {
							card.Background(orbPal.CardTint).Border(1.5, orbPal.Mid.Alpha(0.50))
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
								ui.Row(c).FillWidth().Gap(8.5).AlignItems(ui.Start).Children(func() {
									ui.Box(c).Padding(2.5, 0, 0, 0).Shrink(0).Children(func() {
										ui.Icon(c, SVGForIconKey(domain.ResolveIdeaIconKey(n))).
											Size(18, 18).TextColor(orbPal.BadgeText)
									})
									ui.Text(c, n.Title).FontSize(FontCardTitle).Bold().LineHeight(1.34).
										TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(2)
								})

								ui.Text(c, n.ConversationalSummary).
									FontSize(FontBody).LineHeight(1.54).TextColor(pal.InkSecondary).MaxLines(2)

								ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Padding(2, 0, 0, 0).Children(func() {
									ui.Row(c).Gap(6).AlignItems(ui.Center).Grow(1).MinWidth(0).Wrap().Children(func() {
										shown := 0
										for _, rawTag := range n.Tags {
											tg := cleanTagLabel(rawTag)
											if tg == "" {
												continue
											}
											ui.Box(c).Padding(2.5, 8).Radius(RadiusTag).
												Background(orbPal.BadgeBg.Alpha(0.82)).
												Children(func() {
													ui.Text(c, tg).FontSize(FontCaption).FontWeight(500).
														TextColor(orbPal.BadgeText).SingleLine()
												})
											shown++
											if shown >= 3 {
												break
											}
										}
									})

									ui.Text(c, formatChineseShortDate(n.UpdatedAt)).FontSize(FontCaption).FontFeatures("tnum").
										TextColor(pal.InkMuted).SingleLine().Shrink(0)
								})
							})
						})
					}
				})
			})

		// Soft bottom canvas fade so cards gently recede above the bottom bar and view-switcher orb
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(42).PassThrough().
			LinearGradient(ui.LinearGradient{
				From:  pal.Canvas.Alpha(0.0),
				To:    pal.Canvas,
				Angle: 180,
				Oklab: true,
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
