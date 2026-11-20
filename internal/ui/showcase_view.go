package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

func (a *App) renderDashboardView(c *ui.Context) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()

	ui.Box(c).Key("space_dashboard_root").Fill().Background(pal.Canvas).Children(func() {
		sc := ui.Scroll(c).Key("space_dashboard").TrackScroll(&a.DashboardScroll).Fill().Background(pal.Canvas).
			HandleInput(func(ev ui.InputEvent) bool {
				// Scrolling down past the bottom of the Dashboard enters the Inspiration Archive Vault.
				if ev.Kind == ui.InputScroll && ev.DY > 18 && a.DashboardScroll.Y >= a.DashboardScroll.MaxY-8 {
					a.enterArchiveWithTag("")
					return true
				}
				return false
			})

		if sc.PullToRefresh(&a.DashboardRefresh) {
			mygo.Haptics.Light()
			a.RunAsync("正在刷新灵感看板…", func(ctx context.Context) error {
				defer a.Dispatch(func() { a.DashboardRefresh.Refreshing = false })
				if a.Deps.AI != nil && a.Deps.AI.Configured() {
					if spark, err := a.Deps.Incubate.SuggestShowcaseSpark(ctx); err == nil && spark != "" {
						a.Dispatch(func() { a.ShowcaseSpark = spark })
					}
				}
				return nil
			})
		}

		sc.Children(func() {
			padX := float32(16)
			padTop := float32(14)
			if !phone {
				padX = 28
				padTop = 20
			}

			inner := func() {
				ui.Column(c).FillWidth().Padding(padTop, padX, 96+bottomSafe, padX).Gap(12).Children(func() {
					a.renderDashboardHeader(c, pal)

					if a.DashboardSearchOpen || strings.TrimSpace(a.SearchQuery) != "" {
						a.renderDashboardSearchCard(c, pal)
					}

					a.renderDashboardHeroRow(c, pal, phone)
					a.renderDashboardKeywordMatrix(c, pal)
					a.renderDashboardCluesCard(c, pal)
					a.renderDashboardArchivePortal(c, pal)
				})
			}

			if phone {
				inner()
				return
			}
			ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Box(c).FillWidth().MaxWidth(760).Children(inner)
			})
		})

		a.renderDashboardCaptureSpeedDial(c, pal, bottomSafe, phone)
	})
}

func (a *App) renderDashboardHeader(c *ui.Context, pal SurfacePalette) {
	winW, _ := c.Size()
	compact := winW > 0 && winW < 380
	gap := float32(6)
	if compact {
		gap = 5
	}

	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(gap).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Shrink(0).Children(func() {
			ui.Box(c).Size(26, 26).Radius(7).Center().Background(pal.Ink).Children(func() {
				ui.Icon(c, IconSpark).Size(14, 14).TextColor(pal.Card)
			})
			ui.Text(c, "Neromancer").FontSize(16).Bold().TextColor(pal.Ink).SingleLine()
		})

		ui.Spacer(c).Grow(1)

		ui.Row(c).Gap(gap).AlignItems(ui.Center).Shrink(0).Children(func() {
			if a.BusyTask != "" {
				ui.Spinner(c).Size(14, 14)
			}

			if IconAction(c, "dash_search_btn", IconSearch, "搜索灵感", a.DashboardSearchOpen).Tooltip("搜索灵感").Clicked() {
				a.CaptureDialOpen = false
				a.DashboardSearchOpen = !a.DashboardSearchOpen
				if !a.DashboardSearchOpen && strings.TrimSpace(a.SearchQuery) == "" {
					a.RecallHits = nil
				}
			}

			ui.Box(c).Key("dash_sleep_wrap").Size(32, 32).Shrink(0).Children(func() {
				if IconAction(c, "dash_sleep_btn", IconMoon, "灵感串联", a.SleepModalOpen).Tooltip("灵感串联").Clicked() {
					a.CaptureDialOpen = false
					a.SleepModalOpen = true
				}
				if len(a.SleepInsights) > 0 {
					ui.Box(c).Absolute().Top(4).Right(4).Size(6, 6).Radius(3).
						Background(ColorVermilion).PassThrough()
				}
			})

			configured := a.Deps.AI != nil && a.Deps.AI.Configured()
			ui.Box(c).Key("dash_settings_wrap").Size(32, 32).Shrink(0).Children(func() {
				if IconAction(c, "dash_settings_btn", IconSliders, "DeepSeek 设置", a.SettingsModalOpen).Tooltip("DeepSeek 设置").Clicked() {
					a.CaptureDialOpen = false
					a.SettingsModalOpen = true
				}
				if configured {
					ui.Box(c).Absolute().Top(4).Right(4).Size(6, 6).Radius(3).
						Background(ColorSuccessInk).PassThrough()
				}
			})
		})
	})
}

func (a *App) renderDashboardSearchCard(c *ui.Context, pal SurfacePalette) {
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Row(c).FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
			Radius(12).Background(pal.Card).Border(1, pal.Border).
			Role(ui.RoleSearch).
			Children(func() {
				ui.Icon(c, IconSearch).Size(15, 15).TextColor(pal.InkMuted).Shrink(0)
				if ui.TextInputBase(c, &a.SearchQuery).
					Placeholder("搜搜以前记过的想法…").
					PlaceholderColor(pal.InkMuted).
					TextColor(pal.Ink).
					FontSize(13.5).
					Label("搜索灵感").
					Search().
					Grow(1).MinWidth(0).
					Changed() {
					a.refreshRecallHits()
				}
				if strings.TrimSpace(a.SearchQuery) != "" || a.DashboardSearchOpen {
					if GhostIconAction(c, "dash_search_clear", IconClose, "清空搜索", false).
						Size(26, 26).Clicked() {
						a.SearchQuery = ""
						a.RecallHits = nil
						a.DashboardSearchOpen = false
					}
				}
			})

		if strings.TrimSpace(a.SearchQuery) != "" {
			a.renderRecallResults(c)
		}
	})
}

func (a *App) refreshRecallHits() {
	q := strings.TrimSpace(a.SearchQuery)
	if q == "" {
		a.RecallHits = nil
		return
	}
	hits, err := a.Deps.Organize.SearchRecall(q, 4)
	if err == nil {
		a.RecallHits = hits
	}
}

func (a *App) renderRecallResults(c *ui.Context) {
	pal := a.Palette()

	ui.Column(c).FillWidth().Padding(12).Gap(8).Radius(14).
		Background(pal.Card).Border(1, pal.Border).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, IconSearch).Size(14, 14).TextColor(pal.Ink).Shrink(0)
				ui.Textf(c, "找到 %d 条相关记录", len(a.RecallHits)).
					FontSize(12.5).Bold().TextColor(pal.Ink).Grow(1)
				if PillAction(c, "recall_clear", IconClose, "清除", false).Clicked() {
					a.SearchQuery = ""
					a.RecallHits = nil
				}
			})

			if len(a.RecallHits) == 0 {
				ui.Text(c, "还没有关于这个词的记录，随手记一笔吧。").
					FontSize(12).TextColor(pal.InkSecondary)
				return
			}

			for i, hit := range a.RecallHits {
				h := hit
				row := ui.ButtonBase(c).Key(fmt.Sprintf("recall_%d_%s", i, h.Node.ID)).
					FillWidth().Padding(9, 10).Radius(11).Background(pal.SoftPanel).
					Cursor(ui.CursorPointer).Label(h.Node.Title)
				mobile.Ink(row)
				row.OnClick(func() {
					mygo.Haptics.Selection()
					a.openNodeDetail(c, h.Node.ID)
				}).Children(func() {
					ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
						IdeaIconBadge(c, domain.ResolveIdeaIconKey(h.Node), h.Node.Theme, IconBadgeProps{
							Size:     30,
							IconSize: 15,
							Radius:   8,
						})
						ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
							ui.Text(c, h.Node.Title).FontSize(13).Bold().TextColor(pal.Ink).MaxLines(1)
							ui.Textf(c, "%s · %d 条原记录", h.MatchReason, h.EvidenceCount).
								FontSize(11).TextColor(pal.InkMuted).MaxLines(1)
						})
						ui.Icon(c, IconChevronRight).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
					})
				})
			}
		})
}

func (a *App) renderDashboardHeroRow(c *ui.Context, pal SurfacePalette, phone bool) {
	digest := a.DashboardDigest
	winW, _ := c.Size()

	rightW := float32(124)
	if !phone {
		rightW = 168
	} else if winW > 0 && winW < 380 {
		rightW = 112
	}

	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Children(func() {
		// Left card: AI Persona Insight
		ui.Column(c).Grow(1).Basis(0).Padding(14).Gap(8).Radius(16).
			Background(pal.Card).Border(1, pal.Border).Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Padding(2, 7).Radius(6).Background(ColorSuccessSoft).Children(func() {
					ui.Text(c, "AI 灵感画像").FontSize(10.5).FontWeight(700).TextColor(ColorSuccessInk).SingleLine()
				})
				if a.Deps.AI != nil && a.Deps.AI.Configured() {
					if GhostIconAction(c, "dash_spark_refresh", IconRefresh, "刷新洞察", false).
						Size(22, 22).Clicked() {
						a.RunAsync("正在刷新灵感看板…", func(ctx context.Context) error {
							spark, err := a.Deps.Incubate.SuggestShowcaseSpark(ctx)
							if err != nil {
								return err
							}
							a.Dispatch(func() { a.ShowcaseSpark = spark })
							return nil
						})
					}
				}
			})

			ui.Text(c, digest.PersonaHeadline).FontSize(16).Bold().LineHeight(1.28).
				TextColor(pal.Ink).MaxLines(2)

			summaryText := digest.PersonaSummary
			if strings.TrimSpace(a.ShowcaseSpark) != "" {
				summaryText = a.ShowcaseSpark
			}
			ui.Text(c, summaryText).FontSize(12).LineHeight(1.48).
				TextColor(pal.InkSecondary).MaxLines(3)
		})

		// Right card: Pulse-style Weekly Metric & Sparkline
		metricBtn := ui.ButtonBase(c).Key("dash_metric_tile").
			Width(rightW).Shrink(0).Padding(14).Radius(16).
			Background(pal.Card).Border(1, pal.Border).
			Cursor(ui.CursorPointer).Label("打开灵感档案室")
		mobile.Ink(metricBtn)
		metricBtn.OnClick(func() {
			mygo.Haptics.Selection()
			a.enterArchiveWithTag("")
		}).Children(func() {
			ui.Column(c).Fill().Justify(ui.SpaceBetween).Gap(6).Children(func() {
				ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "本周积累").FontSize(11).FontWeight(600).TextColor(pal.InkMuted).SingleLine()
					ui.Box(c).Size(6, 6).Radius(3).Background(ColorSuccessInk).Shrink(0)
				})

				ui.Row(c).FillWidth().AlignItems(ui.End).Gap(4).Children(func() {
					ui.Text(c, fmt.Sprintf("%02d", digest.TotalNodes)).
						FontSize(28).Bold().FontFeatures("tnum").TextColor(pal.Ink).SingleLine()
					ui.Textf(c, "篇 / %d 条", digest.TotalEvidences).
						FontSize(10.5).Padding(0, 0, 4, 0).TextColor(pal.InkMuted).SingleLine().MinWidth(0)
				})

				ptsNorm := digest.Sparkline
				ui.Box(c).FillWidth().Height(26).Draw(func(p *ui.Painter, r ui.Rect) {
					drawDashboardSparkline(p, r, ptsNorm, ColorSuccessInk)
				})
			})
		})
	})
}

func drawDashboardSparkline(p *ui.Painter, r ui.Rect, values []float32, strokeCol ui.Color) {
	if p == nil || r.W < 12 || r.H < 8 || len(values) < 2 {
		return
	}
	pts := make([][2]float32, len(values))
	padX := float32(2)
	usableW := max(float32(4), r.W-padX*2)
	for i, v := range values {
		t := float32(i) / float32(len(values)-1)
		clamped := max(float32(0.08), min(float32(0.95), v))
		x := r.X + padX + usableW*t
		y := r.Y + r.H*(1.0-clamped)
		pts[i] = [2]float32{x, y}
	}
	p.StrokePolyline(pts, 2.0, strokeCol)
	last := pts[len(pts)-1]
	var dot ui.Path
	dot.Circle(last[0], last[1], 2.5)
	p.FillPath(&dot, strokeCol)
}

func (a *App) renderDashboardKeywordMatrix(c *ui.Context, pal SurfacePalette) {
	clusters := a.DashboardDigest.Clusters

	ui.Column(c).FillWidth().Padding(14).Gap(11).Radius(16).
		Background(pal.Card).Border(1, pal.Border).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "AI 提炼关键词矩阵").FontSize(13.5).Bold().TextColor(pal.Ink)
			ui.Text(c, "点按过滤档案").FontSize(11).TextColor(pal.InkMuted)
		})

		if len(clusters) == 0 {
			ui.Text(c, "记下几条想法后，这里会自动提炼关键词。").
				FontSize(12).TextColor(pal.InkSecondary).Padding(4, 0)
			return
		}

		for rowStart := 0; rowStart < len(clusters); rowStart += 2 {
			start := rowStart
			ui.Row(c).FillWidth().Gap(9).Children(func() {
				for col := 0; col < 2; col++ {
					idx := start + col
					if idx >= len(clusters) {
						ui.Box(c).Grow(1).Basis(0)
						continue
					}
					a.renderKeywordClusterTile(c, pal, idx, clusters[idx])
				}
			})
		}
	})
}

func (a *App) renderKeywordClusterTile(c *ui.Context, pal SurfacePalette, idx int, cl service.KeywordCluster) {
	orbPal := ColorsForTheme(cl.Theme)
	share := max(float32(0.15), min(float32(1.0), cl.Share))

	tile := ui.ButtonBase(c).Key(fmt.Sprintf("kw_tile_%d_%s", idx, cl.FilterTag)).
		Grow(1).Basis(0).Padding(11, 12).Radius(12).
		Background(pal.SoftPanel).Border(1, pal.Border.Alpha(0.75)).
		Cursor(ui.CursorPointer).Label(cl.Name)
	mobile.Ink(tile)
	tile.OnClick(func() {
		mygo.Haptics.Selection()
		a.enterArchiveWithTag(cl.FilterTag)
	}).Children(func() {
		ui.Column(c).FillWidth().Gap(7).Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				IdeaIconBadge(c, cl.IconKey, cl.Theme, IconBadgeProps{
					Size:     26,
					IconSize: 14,
					Radius:   7,
				})
				ui.Textf(c, "%d%%", cl.Percent).
					FontSize(11.5).Bold().FontFeatures("tnum").TextColor(orbPal.BadgeText).SingleLine()
			})

			ui.Column(c).FillWidth().Gap(2).Children(func() {
				ui.Text(c, cl.Name).FontSize(13).Bold().TextColor(pal.Ink).SingleLine()
				ui.Text(c, cl.SubKeywords).FontSize(10.5).TextColor(pal.InkMuted).SingleLine()
			})

			ui.Box(c).FillWidth().Height(4).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Fill(r, pal.Border, 2)
				fillW := max(float32(6), r.W*share)
				p.Fill(ui.Rect{X: r.X, Y: r.Y, W: fillW, H: r.H}, orbPal.Mid, 2)
			})
		})
	})
}

func (a *App) renderDashboardCluesCard(c *ui.Context, pal SurfacePalette) {
	clues := a.DashboardDigest.Clues

	ui.Column(c).FillWidth().Padding(14).Gap(9).Radius(16).
		Background(pal.Card).Border(1, pal.Border).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "串联与待深化追问").FontSize(13.5).Bold().TextColor(pal.Ink)
			ui.Textf(c, "%d 条新线索", len(clues)).FontSize(11).FontWeight(600).TextColor(ui.Hex("#D97706"))
		})

		if len(clues) == 0 {
			ui.Text(c, "暂无新线索，记录更多想法或点击右上角「灵感串联」试试。").
				FontSize(12).TextColor(pal.InkSecondary).Padding(2, 0)
			return
		}

		for i, clue := range clues {
			cl := clue
			orbPal := ColorsForTheme(cl.Theme)
			rowBtn := ui.ButtonBase(c).Key(fmt.Sprintf("dash_clue_%d_%s", i, cl.Kind)).
				FillWidth().Padding(9, 11).Radius(10).
				Background(pal.SoftPanel).Cursor(ui.CursorPointer).Label(cl.Text)
			mobile.Ink(rowBtn)
			rowBtn.OnClick(func() {
				mygo.Haptics.Selection()
				if cl.Kind == "sleep" {
					a.SleepModalOpen = true
					return
				}
				if cl.NodeID != "" {
					a.openNodeDetail(c, cl.NodeID)
				}
			}).Children(func() {
				ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, SVGForIconKey(cl.IconKey)).Size(15, 15).TextColor(orbPal.BadgeText).Shrink(0)
					ui.Text(c, cl.Text).FontSize(12).TextColor(pal.Ink).Grow(1).MinWidth(0).SingleLine()
					ui.Icon(c, IconChevronRight).Size(13, 13).TextColor(pal.InkMuted).Shrink(0)
				})
			})
		}
	})
}

func (a *App) renderDashboardArchivePortal(c *ui.Context, pal SurfacePalette) {
	portal := ui.ButtonBase(c).Key("dash_enter_archive").
		FillWidth().Padding(11, 14).Radius(12).
		Background(ui.Hex("#E7E9EF")).Cursor(ui.CursorPointer).Label("进入灵感档案室")
	mobile.Ink(portal)
	portal.OnClick(func() {
		mygo.Haptics.Selection()
		a.enterArchiveWithTag("")
	}).Children(func() {
		ui.Row(c).FillWidth().Center().Gap(6).Children(func() {
			ui.Icon(c, IconArrowDown).Size(13, 13).TextColor(pal.InkSecondary).Shrink(0)
			ui.Textf(c, "下滑打开「灵感档案室」（%d 篇归档）", len(a.Nodes)).
				FontSize(12).FontWeight(600).TextColor(pal.InkSecondary).SingleLine()
		})
	})
}

func trimText(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "…"
}
