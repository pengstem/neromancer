package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
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
						a.renderDashboardWordCloudGlobe(c, pal)
						a.renderDashboardCluesCard(c, pal)
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

	pullLabel := fmt.Sprintf("下拉打开思维宫殿 · %d", len(a.Nodes))
	if compact {
		pullLabel = "下拉打开思维宫殿"
	}
	if ready {
		pullLabel = "松手打开思维宫殿"
	} else if pullDist > 6 {
		pullLabel = "继续下拉打开思维宫殿"
	}

	pillBg := pal.Card
	pillBorder := pal.Border.Alpha(0.75)
	pillFg := pal.InkSecondary
	grabberCol := pal.InkMuted.Alpha(0.45 * progress)
	if ready {
		pillBg = ColorVermilion.Alpha(0.10)
		pillBorder = ColorVermilion.Alpha(0.35)
		pillFg = ColorVermilion
		grabberCol = ColorVermilion.Alpha(0.80)
	}

	headerCol := ui.Column(c).Key("dash_top_pull_header").FillWidth().Padding(11, padX, 8, padX).Gap(5)
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
			portal := ui.ButtonBase(c).Key("dash_enter_archive").
				Padding(6, 12).Radius(RadiusPill).
				Background(pillBg).Border(1, pillBorder).
				Shadow(0, 1, 4, 0, ui.RGBA(18, 22, 33, 0.03)).
				Cursor(ui.CursorPointer).
				Shrink(1).MinWidth(0).
				Label("进入思维宫殿")
			mobile.Ink(portal)
			portal.OnClick(func() {
				mygo.Haptics.Selection()
				a.enterArchiveWithTag("")
			}).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
					ui.Icon(c, IconChevronDown).Size(14, 14).TextColor(pillFg).Shrink(0)
					ui.Text(c, pullLabel).
						FontSize(FontMeta).FontWeight(600).TextColor(pillFg).SingleLine().MinWidth(0)
				})
			})

			ui.Spacer(c).Grow(1)

			ui.Row(c).Gap(gap).AlignItems(ui.Center).Shrink(0).Children(func() {
				if a.BusyTask != "" {
					ui.Spinner(c).Size(14, 14)
				}

				ui.Box(c).Key("dash_sleep_wrap").Size(32, 32).Shrink(0).Children(func() {
					if GhostIconAction(c, "dash_sleep_btn", IconMoon, "灵感串联", a.SleepModalOpen).Tooltip("灵感串联").Clicked() {
						a.closeOrbDials()
						a.SleepModalOpen = true
					}
					if len(a.SleepInsights) > 0 {
						ui.Box(c).Absolute().Top(5).Right(5).Size(6, 6).Radius(3).
							Background(ColorVermilion).PassThrough()
					}
				})

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
				ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "本周积累").FontSize(FontCaption).FontWeight(600).TextColor(pal.InkMuted).SingleLine()
					ui.Box(c).Size(6, 6).Radius(3).Background(ColorVermilion.Alpha(0.75)).Shrink(0)
				})

				ui.Row(c).FillWidth().AlignItems(ui.End).Gap(4).Children(func() {
					ui.Text(c, fmt.Sprintf("%02d", digest.TotalNodes)).
						FontSize(FontMetric).Bold().FontFeatures("tnum").TextColor(pal.Ink).SingleLine()
					ui.Textf(c, "篇 / %d 条", digest.TotalEvidences).
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

func (a *App) renderDashboardCluesCard(c *ui.Context, pal SurfacePalette) {
	clues := a.DashboardDigest.Clues
	badgeCol := pal.InkMuted
	badgeBg := pal.SoftPanel
	if len(clues) > 0 {
		badgeCol = ColorVermilion
		badgeBg = ColorDangerSoft
	}

	StyleSurfaceCard(
		ui.Column(c).FillWidth().Padding(14, 16).Gap(6),
		pal,
	).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.SpaceBetween).AlignItems(ui.Center).Padding(0, 0, 4, 0).Children(func() {
			ui.Text(c, "串联与待深化追问").FontSize(FontBody).Bold().TextColor(pal.Ink)
			ui.Textf(c, "%d 条新线索", len(clues)).FontSize(FontCaption).FontWeight(600).
				FontFeatures("tnum").Padding(2, 8).Radius(RadiusPill).
				Background(badgeBg).TextColor(badgeCol)
		})

		if len(clues) == 0 {
			ui.Text(c, "暂无新线索，记录更多想法或点击右上角「灵感串联」试试。").
				FontSize(FontMeta).TextColor(pal.InkSecondary).Padding(4, 0)
			return
		}

		for i, clue := range clues {
			cl := clue
			orbPal := ColorsForTheme(cl.Theme)
			if i > 0 {
				ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.55))
			}
			rowBtn := ui.ButtonBase(c).Key(fmt.Sprintf("dash_clue_%d_%s", i, cl.Kind)).
				FillWidth().Padding(9, 2).Radius(RadiusControl).
				Background(ui.Transparent).Cursor(ui.CursorPointer).Label(cl.Text)
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
					ui.Icon(c, SVGForIconKey(cl.IconKey)).Size(16, 16).TextColor(orbPal.BadgeText).Shrink(0)
					ui.Text(c, cl.Text).FontSize(FontBody).TextColor(pal.Ink).Grow(1).MinWidth(0).SingleLine()
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
