package ui

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"os"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/markdown"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

func (a *App) renderDetailSheet(c *ui.Context) {
	if !a.isPhoneLayout(c) {
		a.DetailSheetOpen = false
		return
	}
	pal := a.Palette()
	sheetBg := pal.Card
	mobile.Sheet(c, &a.DetailSheetOpen, mobile.SheetOptions{
		Detents:    []float32{0.82, 0.95},
		Background: &sheetBg,
		Label:      "灵感详情",
	}, func() {
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
			// Fixed non-scrolling header keeps sheet grabber clearance & gives an explicit close button.
			ui.Row(c).FillWidth().Padding(10, 18, 10, 18).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, "灵感详情").FontSize(12.5).FontWeight(600).LetterSpacing(0.4).TextColor(pal.InkMuted)
				if GhostIconAction(c, "detail_close", IconClose, "关闭", false).Clicked() {
					a.DetailSheetOpen = false
				}
			})
			ui.Box(c).FillWidth().Height(1).Background(pal.Border)
			a.renderDetailContent(c)
		})
	})
}

func (a *App) renderDetailContent(c *ui.Context) {
	pal := a.Palette()

	node, ok := a.SelectedNode()
	if !ok {
		ui.Column(c).Fill().Center().Padding(28, 28, 48, 28).Gap(10).Children(func() {
			IdeaIconBadge(c, domain.IconKeyBulb, domain.ThemeAmber, IconBadgeProps{
				Size:     48,
				IconSize: 24,
				Radius:   13,
			})
			ui.Text(c, "点选一条灵感查看详情").FontSize(16).Bold().TextColor(pal.Ink)
			ui.Text(c, "整理后的摘要、追问和最初的原记录都在这里。").
				FontSize(13).TextColor(pal.InkSecondary).TextAlign(ui.Center)
		})
		return
	}

	orbPal := ColorsForTheme(node.Theme)
	softTint, badgeBg, badgeText := mindOrbSurface(pal, node.Theme)
	evCount := len(a.SelectedEvidences)

	ui.Scroll(c).TrackScroll(&a.DetailScroll).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(16, 18, 36, 18).Gap(18).Children(func() {
			// Top Identity Hero: 48px auto-matched SVG badge + 20px Bold Title + date/records caption
			ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
				IdeaIconBadge(c, domain.ResolveIdeaIconKey(node), node.Theme, IconBadgeProps{
					Size:     48,
					IconSize: 24,
					Radius:   13,
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, node.Title).FontSize(20).Bold().LineHeight(1.28).TextColor(pal.Ink).MaxLines(3)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Text(c, formatChineseShortDate(node.UpdatedAt)).
							FontSize(12).FontFeatures("tnum").TextColor(pal.InkMuted)
						ui.Text(c, "·").FontSize(12).TextColor(pal.InkMuted)
						ui.Textf(c, "%d 条原记录", evCount).FontSize(12).TextColor(pal.InkMuted)
					})
				})
			})

			// Crisp Top Action Bar: "原始记录" (with count pill) & "分享"
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				evBtn := ui.ButtonBase(c).Key("detail_raw_ev").
					Padding(8, 13).Radius(999).Gap(7).AlignItems(ui.Center).Shrink(0).
					Background(pal.Ink).Cursor(ui.CursorPointer).Label("原始记录")
				mobile.Ink(evBtn)
				evBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.EvidenceModalOpen = true
				}).Children(func() {
					ui.Icon(c, IconArchive).FontSize(14).TextColor(pal.Card)
					ui.Text(c, "原始记录").FontSize(13).FontWeight(600).TextColor(pal.Card).SingleLine()
					ui.Textf(c, "%d", evCount).FontSize(11).FontWeight(700).
						Padding(1, 6).Radius(999).
						Background(pal.Card.Alpha(0.18)).TextColor(pal.Card).SingleLine()
				})

				if PillAction(c, "detail_share", IconShare, "分享", false).Shrink(0).Clicked() {
					mygo.Haptics.Selection()
					a.ShareNodeID = node.ID
					a.ShareModalOpen = true
				}
			})

			// Tag row & separate italic visual caption row
			if len(node.Tags) > 0 {
				ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
					for i, tag := range node.Tags {
						t := tag
						ui.Text(c, "#"+t).Key(fmt.Sprintf("dtag_%d_%s", i, t)).FontSize(11).FontWeight(500).
							Padding(3, 10).Radius(999).
							Background(badgeBg).TextColor(badgeText)
					}
				})
			}
			if capText := strings.TrimSpace(node.Visual.Caption); capText != "" {
				ui.Text(c, capText).FontSize(12).Italic().TextColor(pal.InkMuted)
			}

			// Editorial Spoken Summary: 3px warm left accent bar + conversational prose
			ui.Row(c).FillWidth().Gap(14).Padding(14, 16).Radius(14).
				Background(softTint).AlignItems(ui.Stretch).
				Children(func() {
					ui.Box(c).Width(3).Radius(2).Background(orbPal.Mid).Shrink(0)
					ui.Text(c, node.ConversationalSummary).
						FontSize(15).LineHeight(1.65).TextColor(pal.Ink).Grow(1).MinWidth(0)
				})

			// "再想一步" Socratic question cards
			a.renderGuidingQuestionsSection(c, pal, badgeText, node)

			// On-demand "AI 帮忙" section
			a.renderActionPlanSection(c, pal, node)
		})
	})
}

func (a *App) renderGuidingQuestionsSection(c *ui.Context, pal SurfacePalette, accentNumCol ui.Color, node domain.IdeaNode) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "再想一步").FontSize(14).Bold().TextColor(pal.Ink).Grow(1)
			if PillAction(c, "detail_refresh_q", IconRefresh, "换个角度", false).Shrink(0).Clicked() {
				nodeID := node.ID
				a.RunAsync("正在想几个新角度…", func(ctx context.Context) error {
					_, err := a.Deps.Incubate.RefreshGuidingQuestions(ctx, nodeID)
					return err
				})
			}
		})

		if len(node.GuidingQuestions) == 0 {
			ui.Text(c, "还没有追问，点右上角「换个角度」让 AI 陪你往下想。").
				FontSize(13).LineHeight(1.5).TextColor(pal.InkSecondary).
				Padding(12, 14).Radius(12).Background(pal.SoftPanel.Alpha(0.55))
			return
		}

		for i, q := range node.GuidingQuestions {
			ui.Row(c).Key(fmt.Sprintf("gq_%d", i)).FillWidth().
				Padding(12, 14).Gap(10).Radius(12).
				Background(pal.SoftPanel.Alpha(0.65)).
				Border(1, pal.Border.Alpha(0.6)).
				AlignItems(ui.Start).
				Children(func() {
					ui.Textf(c, "0%d", i+1).FontSize(11).FontWeight(700).
						TextColor(accentNumCol).Padding(2, 0, 0, 0).Shrink(0)
					ui.Text(c, q).FontSize(13).LineHeight(1.55).TextColor(pal.Ink).Grow(1).MinWidth(0)
				})
		}
	})
}

func (a *App) renderActionPlanSection(c *ui.Context, pal SurfacePalette, node domain.IdeaNode) {
	ui.Column(c).FillWidth().Padding(14, 16).Gap(12).Radius(16).
		Background(pal.Canvas).Border(1, pal.Border).
		Children(func() {
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Grow(1).MinWidth(0).Gap(7).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconSpark).FontSize(15).TextColor(pal.Accent)
					ui.Text(c, "行动方案").FontSize(14).Bold().TextColor(pal.Ink)
				})
				if PillAction(c, "detail_ai_plan", IconSpark, "AI 帮忙", true).Shrink(0).Clicked() {
					nodeID := node.ID
					a.RunAsync("正在帮你整理行动步骤…", func(ctx context.Context) error {
						_, err := a.Deps.Incubate.GenerateActionPlan(ctx, nodeID)
						return err
					})
				}
			})

			if strings.TrimSpace(node.ActionPlanMD) != "" {
				ui.Divider(c)
				markdown.View(c, node.ActionPlanMD, &a.MarkdownOpts).FillWidth()
			}
		})
}

func (a *App) renderEvidenceModal(c *ui.Context) {
	pal := a.Palette()
	node, ok := a.SelectedNode()
	if !ok {
		a.EvidenceModalOpen = false
	}

	sheetBg := pal.Card
	mobile.Sheet(c, &a.EvidenceModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.94, 0.78},
		Background: &sheetBg,
		Label:      "原始记录",
	}, func() {
		if !ok {
			return
		}
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Padding(14, 18, 16, 18).Gap(10).Children(func() {
			// Header: "原始记录" + count + quiet close icon button
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconArchive).FontSize(16).TextColor(pal.Ink)
						ui.Text(c, "原始记录").FontSize(18).Bold().TextColor(pal.Ink)
						ui.Textf(c, "%d 条", len(a.SelectedEvidences)).FontSize(11).FontWeight(600).
							Padding(2, 8).Radius(999).Background(pal.SoftPanel).TextColor(pal.InkSecondary)
					})
					ui.Text(c, node.Title).FontSize(12).TextColor(pal.InkMuted).SingleLine()
				})

				if GhostIconAction(c, "ev_close", IconClose, "关闭", false).Clicked() {
					a.EvidenceModalOpen = false
				}
			})

			ui.Divider(c)

			// Immutable RawEvidence archive tape
			ui.Scroll(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Gap(12).Padding(2, 0, 8, 0).Children(func() {
					for i, ev := range a.SelectedEvidences {
						item := ev
						modIcon, modLabel := detailModalityMeta(item.Modality)

						ui.Column(c).Key("ev_item_"+item.ID).FillWidth().
							Padding(14).Gap(10).Radius(14).
							Background(pal.Canvas).Border(1, pal.Border).
							Children(func() {
								// Modality + timestamp + scene metadata row
								ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
									ui.Row(c).Gap(4).Padding(2, 8).Radius(999).
										Background(pal.SoftPanel).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, modIcon).FontSize(11).TextColor(pal.Accent)
										ui.Textf(c, "#%d %s", i+1, modLabel).
											FontSize(11).FontWeight(600).TextColor(pal.Accent)
									})

									ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, IconClock).FontSize(11).TextColor(pal.InkMuted)
										ui.Text(c, item.CapturedAt.Format("2006-01-02 15:04")).
											FontSize(11).TextColor(pal.InkMuted)
									})

									ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, IconMapPin).FontSize(11).TextColor(pal.InkMuted)
										ui.Text(c, item.Scene.LocationSummaryLine()).
											FontSize(11).TextColor(pal.InkSecondary)
									})
								})

								// Verbatim RawText quote block
								if strings.TrimSpace(item.RawText) != "" {
									ui.Row(c).FillWidth().Gap(10).Padding(10, 12).Radius(10).
										Background(pal.Card).AlignItems(ui.Stretch).
										Children(func() {
											ui.Box(c).Width(2.5).Radius(2).Background(pal.Accent.Alpha(0.6)).Shrink(0)
											ui.Text(c, item.RawText).FontSize(14).LineHeight(1.55).
												TextColor(pal.Ink).Grow(1).MinWidth(0)
										})
								}

								// Voice/Audio waveform strip + play button
								if item.Modality == "voice" || item.AudioPath != "" {
									ui.Row(c).FillWidth().Gap(10).Padding(8, 10).Radius(12).
										Background(pal.SoftPanel.Alpha(0.75)).AlignItems(ui.Center).
										Children(func() {
											if PillAction(c, "play_"+item.ID, IconPlay, "播放原声", false).Shrink(0).Clicked() {
												if err := a.Deps.Media.PlayAudio(item.AudioPath); err != nil {
													a.setErrorBanner(err.Error())
												} else {
													a.setBanner("正在播放当时的录音")
												}
											}

											evID := item.ID
											barCol := pal.Accent.Alpha(0.78)
											ui.Box(c).Grow(1).Height(26).MinWidth(60).Draw(func(p *ui.Painter, r ui.Rect) {
												drawEvidenceWaveform(p, r, evID, barCol)
											})

											if item.DurationMs > 0 {
												ui.Textf(c, "%.1fs", float64(item.DurationMs)/1000.0).
													FontSize(11).FontWeight(500).TextColor(pal.InkMuted).Shrink(0)
											}
										})
								}

								// Polaroid image preview or graceful placeholder when image modality has no local file
								if item.ImagePath != "" {
									if src := a.loadCachedImage(item.ImagePath); src != nil {
										ui.Column(c).Padding(8, 8, 12, 8).Radius(10).
											Background(pal.Card).Border(1, pal.Border).
											Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.06)).
											Children(func() {
												ui.Image(c, src).Size(156, 112).Radius(6).Fit(ui.Contain)
											})
									} else if item.Modality == "image" {
										ui.Row(c).FillWidth().Padding(9, 12).Gap(8).Radius(10).AlignItems(ui.Center).
											Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
											ui.Icon(c, IconImage).Size(15, 15).TextColor(pal.InkMuted).Shrink(0)
											ui.Text(c, "图片未附带本地文件（仅留存文字描述）").
												FontSize(12).TextColor(pal.InkMuted).Grow(1).MinWidth(0)
										})
									}
								} else if item.Modality == "image" {
									ui.Row(c).FillWidth().Padding(9, 12).Gap(8).Radius(10).AlignItems(ui.Center).
										Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
										ui.Icon(c, IconImage).Size(15, 15).TextColor(pal.InkMuted).Shrink(0)
										ui.Text(c, "图片未附带本地文件（仅留存文字描述）").
											FontSize(12).TextColor(pal.InkMuted).Grow(1).MinWidth(0)
									})
								}
							})
					}
				})
			})

			ui.Divider(c)

			// Append-only composer row
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &a.AppendEvidenceText).
					Placeholder("再补一句当时的想法…").Grow(1)
				if PillAction(c, "ev_append", IconPlus, "补充", true).Shrink(0).Clicked() && strings.TrimSpace(a.AppendEvidenceText) != "" {
					text := a.AppendEvidenceText
					a.AppendEvidenceText = ""
					_, err := a.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
						RawText:      text,
						Modality:     "text",
						TargetNodeID: node.ID,
					})
					if err != nil {
						a.setErrorBanner(err.Error())
					} else {
						mygo.Haptics.Success()
						_ = a.ReloadVault()
						a.setBanner("已补上一条新记录")
					}
				}
			})
		})
	})
}

func detailModalityMeta(modality string) (*ui.SVG, string) {
	switch modality {
	case "voice":
		return IconMic, "语音"
	case "image":
		return IconImage, "图片"
	case "sketch":
		return IconQuill, "涂鸦"
	default:
		return IconArchive, "文字"
	}
}

func drawEvidenceWaveform(p *ui.Painter, r ui.Rect, seedID string, col ui.Color) {
	const bars = 24
	if r.W < 24 || r.H < 8 {
		return
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(seedID))
	seed := h.Sum32()

	step := r.W / float32(bars)
	barW := max(float32(2.0), min(float32(3.5), step*0.56))
	cy := r.Y + r.H/2

	for i := 0; i < bars; i++ {
		// Deterministic pseudo-random wave with natural speech cadence envelope
		seed = seed*1664525 + 1013904223
		rand01 := float64(seed&0xFFFF) / 65535.0
		t := float64(i) / float64(bars-1)
		env := 0.55 + 0.45*math.Sin(t*math.Pi)
		wave := 0.25 + 0.70*rand01*env
		barH := max(float32(4.0), r.H*float32(wave))
		x := r.X + float32(i)*step + (step-barW)/2
		p.Fill(ui.Rect{X: x, Y: cy - barH/2, W: barW, H: barH}, col, barW/2)
	}
}

func (a *App) loadCachedImage(path string) ui.ImageSource {
	if src, ok := a.imgCache[path]; ok {
		return src
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src, err := ui.DecodeImage(raw)
	if err != nil {
		return nil
	}
	a.imgCache[path] = src
	return src
}
