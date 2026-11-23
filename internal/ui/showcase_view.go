package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
)

func (a *App) renderDashboardView(c *ui.Context, pullDist, pullThreshold float32) {
	pal := a.Palette()
	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()

	padX := float32(16)
	if !phone {
		padX = 28
	}

	ui.Box(c).Key("space_dashboard_root").Fill().Background(pal.Canvas).Children(func() {
		ui.Column(c).Fill().Children(func() {
			// Top Pull-Down Archive Bar (outside Scroll so dragging down from top always moves the 2-floor deck immediately)
			topBar := func() {
				a.renderDashboardHeader(c, pal, padX, pullDist, pullThreshold)
			}
			if phone {
				topBar()
			} else {
				ui.Column(c).FillWidth().AlignItems(ui.Center).Children(func() {
					ui.Box(c).FillWidth().MaxWidth(760).Children(topBar)
				})
			}

			sc := ui.Scroll(c).Key("space_dashboard").TrackScroll(&a.DashboardScroll).Grow(1).FillWidth().Background(pal.Canvas).
				HandleInput(func(ev ui.InputEvent) bool {
					return a.handleSpaceBoundaryScroll(ev, SpaceArchive, a.DashboardScroll.Y <= 1)
				})

			sc.Children(func() {
				inner := func() {
					ui.Column(c).FillWidth().Padding(4, padX, 96+bottomSafe, padX).Gap(12).Children(func() {
						a.renderDashboardHeroRow(c, pal, phone)
						a.renderDashboardSynthesisCard(c, pal)
						a.renderDashboardWordCloudGlobe(c, pal)
						a.renderDashboardQuestionsCard(c, pal)
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
		})
	})
}

func (a *App) renderDashboardHeader(c *ui.Context, pal SurfacePalette, padX, pullDist, pullThreshold float32) {
	winW, _ := c.Size()
	compact := winW > 0 && winW < 380
	gap := float32(6)
	if compact {
		gap = 5
	}

	progress := float32(0)
	if pullThreshold > 0 {
		progress = min(float32(1), max(float32(0), pullDist/pullThreshold))
	}
	ready := pullDist >= pullThreshold && pullThreshold > 0

	grabberCol := pal.InkMuted.Alpha(0.45 * progress)
	if ready {
		grabberCol = ColorVermilion.Alpha(0.80)
	}

	headerCol := ui.Column(c).Key("dash_top_pull_header").FillWidth().Padding(10, padX, 6, padX).Gap(5)
	if pullDist > 0.5 {
		headerCol.BorderWidth(1, 0, 0, 0).BorderColor(pal.Border.Alpha(0.6))
	}
	headerCol.Children(func() {
		// Only surface the pull-down damping bar once the user actively drags downward,
		// so at rest the Dashboard doesn't look like a half-open bottom sheet.
		if pullDist > 2.0 {
			ui.Box(c).Width(32 + 14*progress).Height(3.5).Radius(2).
				Background(grabberCol).AlignSelf(ui.Center)
		}

		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(gap).Children(func() {
			a.renderDashboardPortalCue(c, pal, pullDist, pullThreshold)

			ui.Spacer(c).Grow(1)

			ui.Row(c).Gap(gap).AlignItems(ui.Center).Shrink(0).Children(func() {
				if a.BusyTask != "" {
					ui.Spinner(c).Size(14, 14)
				}

				configured := a.Deps.AI != nil && a.Deps.AI.Configured()
				ui.Box(c).Key("dash_settings_wrap").Size(32, 32).Shrink(0).Children(func() {
					if GhostIconAction(c, "dash_settings_btn", IconSliders, "DeepSeek 设置", a.SettingsModalOpen).Tooltip("DeepSeek 设置").Clicked() {
						a.closeOrbDials()
						a.SettingsModalOpen = true
					}
					if configured {
						ui.Box(c).Absolute().Top(5).Right(5).Size(6, 6).Radius(3).
							Background(ColorSuccessInk).PassThrough()
					}
				})
			})
		})
	})
}

func (a *App) renderDashboardHeroRow(c *ui.Context, pal SurfacePalette, phone bool) {
	digest := a.DashboardDigest
	winW, _ := c.Size()

	rightW := float32(126)
	if !phone {
		rightW = 168
	} else if winW > 0 && winW < 380 {
		rightW = 114
	}

	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Stretch).Children(func() {
		// Left card: AI Persona Insight harmonized with Vermilion & Obsidian tokens
		StyleSurfaceCard(
			ui.Column(c).Grow(1).Basis(0).Padding(14, 15).Gap(8),
			pal,
		).Children(func() {
			ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(6, 6).Radius(3).Background(ColorVermilion).Shrink(0)
					ui.Text(c, "AI 灵感画像").FontSize(FontCaption).FontWeight(700).TextColor(ColorVermilion).SingleLine()
				})
				if a.Deps.AI != nil && a.Deps.AI.Configured() {
					if GhostIconAction(c, "dash_spark_refresh", IconRefresh, "刷新洞察", false).
						Size(24, 24).Clicked() {
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

			ui.Text(c, digest.PersonaHeadline).FontSize(FontCardTitle).Bold().LineHeight(1.32).
				TextColor(pal.Ink).MaxLines(2)

			summaryText := digest.PersonaSummary
			if strings.TrimSpace(a.ShowcaseSpark) != "" {
				summaryText = a.ShowcaseSpark
			}
			ui.Text(c, summaryText).FontSize(FontMeta).LineHeight(1.48).
				TextColor(pal.InkSecondary).MaxLines(3)
		})

		// Right card: Weekly Metric & Sparkline harmonized with Ink + Vermilion live dot
		metricBtn := StyleSurfaceCard(
			ui.ButtonBase(c).Key("dash_metric_tile").Width(rightW).Shrink(0).Padding(14),
			pal,
		).Cursor(ui.CursorPointer).Label("打开思维宫殿")
		mobile.Ink(metricBtn)
		metricBtn.OnClick(func() {
			mygo.Haptics.Selection()
			a.enterArchiveWithTag("")
		}).Children(func() {
			ui.Column(c).Fill().Justify(ui.SpaceBetween).Gap(6).Children(func() {
				ui.Text(c, "本周积累").FontSize(FontCaption).FontWeight(600).TextColor(pal.InkMuted).SingleLine()

				ui.Row(c).FillWidth().AlignItems(ui.End).Gap(4).Children(func() {
					ui.Text(c, fmt.Sprintf("%02d", digest.TotalNodes)).
						FontSize(FontMetric).Bold().FontFeatures("tnum").TextColor(pal.Ink).SingleLine()
					ui.Text(c, "篇").
						FontSize(FontCaption).Padding(0, 0, 4, 0).TextColor(pal.InkMuted).SingleLine().MinWidth(0)
				})

				ptsNorm := digest.Sparkline
				ui.Box(c).FillWidth().Height(26).Draw(func(p *ui.Painter, r ui.Rect) {
					drawDashboardSparkline(p, r, ptsNorm, pal.Ink.Alpha(0.72), ColorVermilion)
				})
			})
		})
	})
}

func drawDashboardSparkline(p *ui.Painter, r ui.Rect, values []float32, strokeCol, tipCol ui.Color) {
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
	p.StrokePolyline(pts, 1.8, strokeCol)
	last := pts[len(pts)-1]
	var dot ui.Path
	dot.Circle(last[0], last[1], 2.6)
	p.FillPath(&dot, tipCol)
}

func dashQuietTextBtn(c *ui.Context, pal SurfacePalette, key, label string, muted bool) ui.Element {
	b := ui.ButtonBase(c).Key(key)
	face := ui.Transparent
	if b.Pressed() {
		face = pal.Border.Alpha(0.45)
	} else if b.Hovered() {
		face = pal.SoftPanel
	}
	fg := pal.InkSecondary
	if muted {
		fg = pal.InkMuted
	}
	if b.Hovered() || b.Pressed() {
		fg = pal.Ink
	}
	b.Padding(4, 7).Radius(RadiusTag).Background(face).
		Cursor(ui.CursorPointer).Shrink(0).Label(label)
	mobile.Ink(b)
	b.Children(func() {
		ui.Text(c, label).FontSize(FontMeta).FontWeight(500).TextColor(fg).SingleLine()
	})
	return b
}

func (a *App) renderDashboardSynthesisCard(c *ui.Context, pal SurfacePalette) {
	total := len(a.SleepInsights)
	idx := a.SleepPageIdx
	if idx < 0 {
		idx = 0
	}
	if total > 0 && idx >= total {
		idx = total - 1
	}

	runLbl := "找找联系"
	if total > 0 {
		runLbl = "重新串联"
	}

	StyleSurfaceCard(
		ui.Column(c).Key("dash_synthesis_card").FillWidth().Padding(15, 16).Gap(8),
		pal,
	).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "灵感串联").FontSize(FontBody).Bold().TextColor(pal.Ink)
				if total > 1 {
					ui.Textf(c, "%d / %d", idx+1, total).
						FontSize(FontMeta).FontFeatures("tnum").TextColor(pal.InkMuted)
				}
			})

			ui.Row(c).Gap(2).AlignItems(ui.Center).Shrink(0).Children(func() {
				if total > 1 {
					if GhostIconAction(c, "dash_sleep_prev", IconChevronLeft, "上一条串联", false).
						Size(26, 26).Clicked() {
						mygo.Haptics.Selection()
						a.SleepPageIdx = (idx - 1 + total) % total
					}
					if GhostIconAction(c, "dash_sleep_next", IconChevronRight, "下一条串联", false).
						Size(26, 26).Clicked() {
						mygo.Haptics.Selection()
						a.SleepPageIdx = (idx + 1) % total
					}
				}
				if dashQuietTextBtn(c, pal, "dash_sleep_run", runLbl, false).Clicked() {
					mygo.Haptics.Light()
					a.RunAsync("正在翻看你的灵感，寻找有趣的联系…", func(ctx context.Context) error {
						_, err := a.Deps.Incubate.RunSleepIncubation(ctx)
						if err == nil {
							a.Dispatch(func() { a.SleepPageIdx = 0 })
						}
						return err
					})
				}
			})
		})

		if total == 0 {
			ui.Text(c, "暂无串联线索").
				FontSize(FontMeta).TextColor(pal.InkSecondary).Padding(2, 0)
			return
		}

		ins := a.SleepInsights[idx]

		ui.Text(c, ins.Title).
			FontSize(FontCardTitle).Bold().LineHeight(1.36).TextColor(pal.Ink)

		ui.Text(c, ins.SparkSummary).
			FontSize(FontBody).LineHeight(1.56).TextColor(pal.InkSecondary).MaxLines(4)

		var linked []domain.IdeaNode
		for _, nid := range ins.ConnectedNodeIDs {
			if nd, ok := a.NodeByID[nid]; ok {
				linked = append(linked, nd)
				if len(linked) >= 2 {
					break
				}
			}
		}

		ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.55))

		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Grow(1).MinWidth(0).Children(func() {
				for i, nd := range linked {
					node := nd
					if i > 0 {
						ui.Text(c, "·").FontSize(FontMeta).TextColor(pal.InkMuted).Shrink(0)
					}
					linkBtn := ui.ButtonBase(c).Key(fmt.Sprintf("dash_sleep_link_%s_%s", ins.ID, node.ID))
					face := ui.Transparent
					fg := pal.InkSecondary
					if linkBtn.Pressed() {
						face = pal.Border.Alpha(0.45)
						fg = pal.Ink
					} else if linkBtn.Hovered() {
						face = pal.SoftPanel
						fg = pal.Ink
					}
					linkBtn.Padding(3, 5).Radius(RadiusTag).Background(face).
						Cursor(ui.CursorPointer).Shrink(1).MinWidth(0).
						Label("查看关联灵感：" + node.Title)
					mobile.Ink(linkBtn)
					if linkBtn.Clicked() {
						mygo.Haptics.Selection()
						a.openNodeDetail(c, node.ID)
					}
					linkBtn.Children(func() {
						ui.Row(c).Gap(2).AlignItems(ui.Center).MinWidth(0).Children(func() {
							ui.Text(c, node.Title).FontSize(FontMeta).FontWeight(500).
								TextColor(fg).SingleLine().MinWidth(0)
							ui.Icon(c, IconChevronRight).Size(12, 12).TextColor(pal.InkMuted).Shrink(0)
						})
					})
				}
			})

			if dashQuietTextBtn(c, pal, "dash_sleep_dismiss_"+ins.ID, "已读", true).Clicked() {
				mygo.Haptics.Light()
				_ = a.Deps.Store.DismissSleepInsight(ins.ID)
				_ = a.ReloadVault()
			}
		})
	})
}

func (a *App) renderDashboardQuestionsCard(c *ui.Context, pal SurfacePalette) {
	type questionItem struct {
		NodeID   string
		Question string
	}
	var items []questionItem
	for _, n := range a.Nodes {
		if len(n.GuidingQuestions) == 0 {
			continue
		}
		q := strings.TrimSpace(n.GuidingQuestions[0])
		if q == "" {
			continue
		}
		items = append(items, questionItem{
			NodeID:   n.ID,
			Question: q,
		})
		if len(items) >= 3 {
			break
		}
	}

	StyleSurfaceCard(
		ui.Column(c).Key("dash_questions_card").FillWidth().Padding(14, 16).Gap(4),
		pal,
	).Children(func() {
		ui.Box(c).FillWidth().Padding(0, 0, 4, 0).Children(func() {
			ui.Text(c, "待深化追问").FontSize(FontBody).Bold().TextColor(pal.Ink)
		})

		if len(items) == 0 {
			ui.Text(c, "暂无待深化线索").
				FontSize(FontMeta).TextColor(pal.InkSecondary).Padding(4, 0)
			return
		}

		for i, item := range items {
			it := item
			if i > 0 {
				ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.55))
			}
			rowBtn := ui.ButtonBase(c).Key(fmt.Sprintf("dash_q_%s", it.NodeID))
			face := ui.Transparent
			if rowBtn.Pressed() {
				face = pal.Border.Alpha(0.4)
			} else if rowBtn.Hovered() {
				face = pal.SoftPanel
			}
			rowBtn.FillWidth().Padding(9, 2).Radius(RadiusControl).
				Background(face).Cursor(ui.CursorPointer).Label(it.Question)
			mobile.Ink(rowBtn)
			if rowBtn.Clicked() {
				mygo.Haptics.Selection()
				a.openNodeDetail(c, it.NodeID)
			}
			rowBtn.Children(func() {
				ui.Row(c).FillWidth().Gap(9).AlignItems(ui.Center).Children(func() {
					ui.Text(c, it.Question).FontSize(FontBody).TextColor(pal.Ink).Grow(1).MinWidth(0).SingleLine()
					ui.Icon(c, IconChevronRight).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
				})
			})
		}
	})
}

func trimText(s string, maxRunes int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= maxRunes {
		return string(r)
	}
	return string(r[:maxRunes]) + "…"
}
