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
		Detents:    []float32{0.80, 0.95},
		Background: &sheetBg,
		Label:      "灵感详情",
	}, func() {
		a.renderDetailContent(c)
	})
}

func (a *App) renderDetailContent(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)

	node, ok := a.SelectedNode()
	if !ok {
		ui.Column(c).Fill().Center().Padding(28, 28, 112, 28).Gap(10).Children(func() {
			ui.Box(c).Size(44, 44).Draw(func(p *ui.Painter, r ui.Rect) {
				DrawAnimatedMemoryOrb(p, r, domain.ThemeSky, 0.72, phase, 0.5, "orbit", false)
			})
			ui.Text(c, "轻触记忆球展开详情").FontSize(16).Bold().TextColor(pal.Ink)
			ui.Text(c, "口语整理、阶段演化与原始记录会留在这里。").
				FontSize(13).TextColor(pal.InkSecondary).TextAlign(ui.Center)
		})
		return
	}

	orbPal := ColorsForTheme(node.Theme)
	softTint, badgeBg, badgeText := mindOrbSurface(pal, a.DarkMode, node.Theme)
	evCount := len(a.SelectedEvidences)

	ui.Scroll(c).TrackScroll(&a.DetailScroll).Grow(1).Basis(0).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Padding(16, 18, 112, 18).Gap(18).Children(func() {
			// Top Identity Hero: 56px animated shader orb + 20px Bold Title + weight/stage caption
			ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(56, 56).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
					DrawAnimatedMemoryOrb(p, r, node.Theme, node.Weight, phase, float32(node.Visual.Energy), node.Visual.MotifType, true)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(5).Children(func() {
					ui.Text(c, node.Title).FontSize(20).Bold().LineHeight(1.28).TextColor(pal.Ink).MaxLines(3)
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(6, 6).Radius(99).Background(orbPal.Rim).Shrink(0)
						ui.Text(c, node.Stage.Label()).FontSize(12).FontWeight(600).TextColor(badgeText)
						ui.Text(c, "·").FontSize(12).TextColor(pal.InkMuted)
						ui.Textf(c, "权重 %.0f%%", node.Weight*100).FontSize(12).TextColor(pal.InkMuted)
					})
				})
			})

			// Crisp Top Action Bar: "原始证据" (with count pill) & "分享"
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				evBtn := ui.ButtonBase(c).Key("detail_raw_ev").
					Padding(8, 13).Radius(999).Gap(7).AlignItems(ui.Center).Shrink(0).
					Background(pal.Accent).Cursor(ui.CursorPointer).Label("原始证据")
				mobile.Ink(evBtn)
				evBtn.OnClick(func() {
					mygo.Haptics.Selection()
					a.EvidenceModalOpen = true
				}).Children(func() {
					ui.Icon(c, IconArchive).FontSize(14).TextColor(pal.AccentText)
					ui.Text(c, "原始证据").FontSize(13).FontWeight(600).TextColor(pal.AccentText).SingleLine()
					ui.Textf(c, "%d", evCount).FontSize(11).FontWeight(700).
						Padding(1, 6).Radius(999).
						Background(pal.AccentText.Alpha(0.18)).TextColor(pal.AccentText).SingleLine()
				})

				if PillAction(c, "detail_share", IconShare, "分享", false).Shrink(0).Clicked() {
					mygo.Haptics.Selection()
					a.ShareNodeID = node.ID
					a.ShareModalOpen = true
				}
			})

			// Connected 4-stage horizontal stepper track
			a.renderStageStepper(c, pal, orbPal, node)

			// Tag row & subtle italic visual caption
			if len(node.Tags) > 0 || strings.TrimSpace(node.Visual.Caption) != "" {
				ui.Row(c).FillWidth().Wrap().Gap(6).AlignItems(ui.Center).Children(func() {
					for i, tag := range node.Tags {
						t := tag
						ui.Text(c, "#"+t).Key(fmt.Sprintf("dtag_%d_%s", i, t)).FontSize(11).FontWeight(500).
							Padding(3, 10).Radius(999).
							Background(badgeBg).TextColor(badgeText)
					}
					if capText := strings.TrimSpace(node.Visual.Caption); capText != "" {
						ui.Text(c, capText).FontSize(11).Italic().TextColor(pal.InkMuted)
					}
				})
			}

			// Editorial Spoken Summary: 3px warm left accent bar + conversational prose
			ui.Row(c).FillWidth().Gap(14).Padding(14, 16).Radius(14).
				Background(softTint).AlignItems(ui.Stretch).
				Children(func() {
					ui.Box(c).Width(3).Radius(2).Background(orbPal.Rim).Shrink(0)
					ui.Text(c, node.ConversationalSummary).
						FontSize(15).LineHeight(1.65).TextColor(pal.Ink).Grow(1).MinWidth(0)
				})

			// "再想一步" Socratic question cards
			a.renderGuidingQuestionsSection(c, pal, badgeText, node)

			// On-demand "AI 帮忙" section
			a.renderActionPlanSection(c, pal, node)

			// Hairline vertical timeline for StageSnapshots
			a.renderStageTimeline(c, pal, badgeBg, badgeText, orbPal)
		})
	})
}

func (a *App) renderStageStepper(c *ui.Context, pal SurfacePalette, orbPal OrbColorSet, node domain.IdeaNode) {
	curIdx := 0
	for i, st := range domain.AllStages {
		if st == node.Stage {
			curIdx = i
			break
		}
	}

	ui.Row(c).FillWidth().Padding(8, 4).Radius(14).
		Background(pal.SoftPanel.Alpha(0.55)).
		Children(func() {
			for i, st := range domain.AllStages {
				stage := st
				idx := i
				lbl := stage.Label()
				isCurrent := idx == curIdx
				isReached := idx <= curIdx

				stepBtn := ui.ButtonBase(c).Key("stg_"+string(stage)).
					Grow(1).Padding(4, 2).Radius(10).Cursor(ui.CursorPointer).Label(lbl)
				mobile.Ink(stepBtn)
				stepBtn.OnClick(func() {
					if stage == node.Stage {
						return
					}
					mygo.Haptics.Selection()
					_, _, err := a.Deps.Organize.AdvanceStage(node.ID, stage, "手动推进生命周期")
					if err != nil {
						a.setErrorBanner(err.Error())
					} else {
						a.setBanner(fmt.Sprintf("已推进至「%s」并留存阶段快照", lbl))
						_ = a.ReloadVault()
					}
				}).Children(func() {
					ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
						// Hairline track row + stage dot
						ui.Row(c).FillWidth().Height(16).AlignItems(ui.Center).Children(func() {
							leftCol := ui.Transparent
							if idx > 0 {
								if idx <= curIdx {
									leftCol = orbPal.Rim
								} else {
									leftCol = pal.Border
								}
							}
							ui.Box(c).Grow(1).Height(1.5).Background(leftCol)

							ui.Box(c).Size(14, 14).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
								cx, cy := r.X+r.W/2, r.Y+r.H/2
								switch {
								case isCurrent:
									var halo ui.Path
									halo.Circle(cx, cy, 6.8)
									p.FillPath(&halo, orbPal.Rim.Alpha(0.22))
									var core ui.Path
									core.Circle(cx, cy, 4.2)
									p.FillPath(&core, orbPal.Rim)
								case isReached:
									var core ui.Path
									core.Circle(cx, cy, 3.6)
									p.FillPath(&core, orbPal.Rim)
								default:
									var bgCircle ui.Path
									bgCircle.Circle(cx, cy, 3.6)
									p.FillPath(&bgCircle, pal.Card)
									p.StrokePath(&bgCircle, 1.4, pal.Border)
								}
							})

							rightCol := ui.Transparent
							if idx < len(domain.AllStages)-1 {
								if idx < curIdx {
									rightCol = orbPal.Rim
								} else {
									rightCol = pal.Border
								}
							}
							ui.Box(c).Grow(1).Height(1.5).Background(rightCol)
						})

						txtCol := pal.InkMuted
						weight := 400
						if isCurrent {
							txtCol = pal.Ink
							weight = 700
						} else if isReached {
							txtCol = pal.InkSecondary
							weight = 500
						}
						ui.Text(c, lbl).FontSize(11).FontWeight(weight).TextColor(txtCol).SingleLine()
					})
				})
			}
		})
}

func (a *App) renderGuidingQuestionsSection(c *ui.Context, pal SurfacePalette, accentNumCol ui.Color, node domain.IdeaNode) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "再想一步").FontSize(14).Bold().TextColor(pal.Ink).Grow(1)
			if PillAction(c, "detail_refresh_q", IconRefresh, "换个角度", false).Shrink(0).Clicked() {
				nodeID := node.ID
				a.RunAsync("DeepSeek 正在构思追问…", func(ctx context.Context) error {
					_, err := a.Deps.Incubate.RefreshGuidingQuestions(ctx, nodeID)
					return err
				})
			}
		})

		if len(node.GuidingQuestions) == 0 {
			ui.Text(c, "尚未生成追问，轻触右上角可顺着你的切入点发散。").
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
					a.RunAsync("DeepSeek 正在拆解可执行方案…", func(ctx context.Context) error {
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

func (a *App) renderStageTimeline(c *ui.Context, pal SurfacePalette, badgeBg, badgeText ui.Color, orbPal OrbColorSet) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "演进轨迹").FontSize(14).Bold().TextColor(pal.Ink)
			ui.Textf(c, "%d", len(a.SelectedSnapshots)).FontSize(11).FontWeight(600).
				Padding(1, 7).Radius(999).Background(pal.SoftPanel).TextColor(pal.InkSecondary)
		})

		if len(a.SelectedSnapshots) == 0 {
			ui.Text(c, "推进阶段或归并记录时会自动留存快照。").
				FontSize(12).TextColor(pal.InkMuted)
			return
		}

		ui.Column(c).FillWidth().Children(func() {
			for i, snap := range a.SelectedSnapshots {
				s := snap
				isLast := i == len(a.SelectedSnapshots)-1

				ui.Row(c).Key("snap_" + s.ID).FillWidth().Gap(12).AlignItems(ui.Stretch).Children(func() {
					// Left rail: dot + vertical hairline
					ui.Column(c).Width(14).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Box(c).Height(5)
						ui.Box(c).Size(8, 8).Radius(99).Background(orbPal.Rim).Shrink(0)
						if !isLast {
							ui.Box(c).Width(1.5).Grow(1).Margin(4, 0, 0, 0).Background(pal.Border)
						}
					})

					// Right timeline entry
					padBot := float32(14)
					if isLast {
						padBot = 2
					}
					ui.Column(c).Grow(1).MinWidth(0).Padding(0, 0, padBot, 0).Gap(4).Children(func() {
						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, s.Stage.Label()).FontSize(10).FontWeight(600).
								Padding(2, 7).Radius(999).Background(badgeBg).TextColor(badgeText)
							ui.Text(c, s.CreatedAt.Format("01-02 15:04")).
								FontSize(11).TextColor(pal.InkMuted)
						})
						ui.Text(c, s.Title).FontSize(13).FontWeight(600).TextColor(pal.Ink)
						if note := strings.TrimSpace(s.Note); note != "" {
							ui.Text(c, note).FontSize(12).LineHeight(1.45).TextColor(pal.InkSecondary)
						}
					})
				})
			}
		})
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
		Label:      "原始证据档案",
	}, func() {
		if !ok {
			return
		}
		ui.Column(c).Grow(1).Basis(0).MinHeight(0).FillWidth().Padding(14, 18, 16, 18).Gap(10).Children(func() {
			// Header: "原始证据" + count + "关闭"
			ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconArchive).FontSize(16).TextColor(pal.Accent)
						ui.Text(c, "原始证据").FontSize(18).Bold().TextColor(pal.Ink)
						ui.Textf(c, "%d 条", len(a.SelectedEvidences)).FontSize(11).FontWeight(600).
							Padding(2, 8).Radius(999).Background(pal.SoftPanel).TextColor(pal.InkSecondary)
					})
					ui.Text(c, node.Title).FontSize(12).TextColor(pal.InkMuted).SingleLine()
				})

				if PillAction(c, "ev_close", IconClose, "关闭", false).Shrink(0).Clicked() {
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
										ui.Text(c, item.Scene.SummaryLine()).
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
													a.setBanner("正在播放当时录下的原始声音片段")
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

								// Polaroid image preview
								if item.ImagePath != "" {
									if src := a.loadCachedImage(item.ImagePath); src != nil {
										ui.Column(c).Padding(8, 8, 12, 8).Radius(10).
											Background(pal.Card).Border(1, pal.Border).
											Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.06)).
											Children(func() {
												ui.Image(c, src).Size(156, 112).Radius(6).Fit(ui.Contain)
											})
									}
								}
							})
					}
				})
			})

			ui.Divider(c)

			// Append-only composer row
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &a.AppendEvidenceText).
					Placeholder("补充一条原始证据到本灵感…").Grow(1)
				if PillAction(c, "ev_append", IconPlus, "追加", true).Shrink(0).Clicked() && strings.TrimSpace(a.AppendEvidenceText) != "" {
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
						a.setBanner("已追加并原样留存一条新的原始证据")
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
		return IconImage, "图像"
	case "sketch":
		return IconQuill, "涂鸦"
	default:
		return IconArchive, "速记"
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
