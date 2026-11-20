package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/imagepicker"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

func (a *App) renderDashboardCaptureSpeedDial(c *ui.Context, pal SurfacePalette, bottomSafe float32, phone bool) {
	if a.DetailSheetOpen || a.CaptureModalOpen || a.EvidenceModalOpen || a.SettingsModalOpen || a.SleepModalOpen || a.ShareModalOpen {
		return
	}

	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(68 + bottomSafe).PassThrough().
		LinearGradient(ui.LinearGradient{
			From:  pal.Canvas.Alpha(0.0),
			To:    pal.Canvas,
			Angle: 180,
			Oklab: true,
		})

	if a.CaptureDialOpen {
		backdrop := ui.ButtonBase(c).Key("capture_dial_backdrop").
			Absolute().Fill().
			Background(ui.RGBA(242, 243, 246, 0.56)).
			Cursor(ui.CursorDefault).
			Label("收起速记菜单")
		backdrop.OnClick(func() {
			mygo.Haptics.Selection()
			a.CaptureDialOpen = false
		})
	}

	rightPad := float32(18)
	bottomPad := float32(20)
	if !phone {
		rightPad = 28
		bottomPad = 26
	}

	ui.Column(c).Key("dash_capture_speed_dial").
		Absolute().Right(rightPad).Bottom(bottomPad + bottomSafe).
		AlignItems(ui.End).Gap(10).
		Children(func() {
			if a.CaptureDialOpen {
				dialItems := []struct {
					key         string
					label       string
					icon        *ui.SVG
					theme       domain.OrbTheme
					modalityIdx int
				}{
					{"dash_dial_image", "图文", IconImage, domain.ThemeAmber, 2},
					{"dash_dial_text", "文字", IconQuill, domain.ThemeSky, 1},
					{"dash_dial_voice", "语音", IconMic, domain.ThemeMint, 0},
				}

				for _, item := range dialItems {
					idx := item.modalityIdx
					orbPal := ColorsForTheme(item.theme)
					pill := ui.ButtonBase(c).Key(item.key)
					pressTarget := float32(1.0)
					if pill.Pressed() {
						pressTarget = 0.96
					}
					scale := pill.Spring("press", pressTarget)

					pill.Padding(6, 7, 6, 13).Radius(999).
						Background(pal.Card).Border(1, pal.Border).
						Shadow(0, 4, 14, 0, ui.RGBA(20, 21, 25, 0.10)).
						Scale(scale, scale).
						Cursor(ui.CursorPointer).Label(item.label)
					mobile.Ink(pill)
					pill.OnClick(func() {
						mygo.Haptics.Light()
						a.CaptureDialOpen = false
						a.CaptureModalityIdx = idx
						a.CaptureModalOpen = true
					}).Children(func() {
						ui.Row(c).Gap(9).AlignItems(ui.Center).Children(func() {
							ui.Text(c, item.label).FontSize(12.5).Bold().TextColor(pal.Ink).SingleLine()
							ui.Box(c).Size(28, 28).Radius(14).Center().
								Background(orbPal.BadgeBg).
								Children(func() {
									ui.Icon(c, item.icon).Size(14, 14).TextColor(orbPal.BadgeText)
								})
						})
					})
				}
			}

			sealBtn := ui.ButtonBase(c).Key("dash_capture_fab")
			bg := ColorVermilion
			if sealBtn.Pressed() {
				bg = ui.Hex("#A63922")
			} else if sealBtn.Hovered() {
				bg = ui.Hex("#B8422A")
			}

			pressTarget := float32(1.0)
			if sealBtn.Pressed() {
				pressTarget = 0.94
			}
			pressScale := sealBtn.Spring("press", pressTarget)

			rotTarget := float32(0)
			if a.CaptureDialOpen {
				rotTarget = 45
			}
			iconAngle := sealBtn.Spring("dial_rot", rotTarget)

			sealLabel := "记一笔"
			if a.CaptureDialOpen {
				sealLabel = "收起"
			}

			sealBtn.Size(54, 54).Radius(27).Center().
				Background(bg).
				Border(1, ui.RGBA(255, 255, 255, 0.18)).
				Shadow(0, 8, 22, 0, ui.RGBA(200, 75, 49, 0.34)).
				Scale(pressScale, pressScale).
				Cursor(ui.CursorPointer).Label(sealLabel)
			if !a.CaptureDialOpen {
				sealBtn.Tooltip("记一笔")
			}
			mobile.Ink(sealBtn)
			sealBtn.OnClick(func() {
				mygo.Haptics.Light()
				a.CaptureDialOpen = !a.CaptureDialOpen
			}).Children(func() {
				ui.Icon(c, IconPlus).Size(22, 22).TextColor(ui.Hex("#FFFFFF")).Rotate(iconAngle)
			})
		})
}

func (a *App) renderCaptureModal(c *ui.Context) {
	pal := a.Palette()

	mobile.Sheet(c, &a.CaptureModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.78, 0.94},
		Background: &pal.Card,
		Label:      "记一条灵感",
	}, func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(14, 18, 28, 18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "记一条灵感").FontSize(18).Bold().TextColor(pal.Ink).Grow(1)
					if GhostIconAction(c, "capture_close", IconClose, "收起", false).Clicked() {
						a.CaptureModalOpen = false
					}
				})

				a.renderCaptureModalityTabs(c, pal)

				if a.CaptureModalityIdx == 0 {
					a.renderVoiceCaptureBar(c)
				}

				if a.CaptureModalityIdx == 2 {
					a.renderImageCaptureBar(c, pal)
				}

				ui.Box(c).FillWidth().Padding(12, 14).Radius(14).
					Background(pal.SoftPanel).Border(1, pal.Border).
					Children(func() {
						ta := ui.TextAreaBase(c, &a.CaptureText).
							Placeholder("想到什么直接记下来…").
							PlaceholderColor(pal.InkMuted).
							TextColor(pal.Ink).
							Label("灵感内容").
							Height(96).FillWidth()
						if a.CaptureModalityIdx == 1 {
							ta.AutoFocus()
						}
						if ta.Changed() {
							if hits, err := a.Deps.Organize.SearchRecall(a.CaptureText, 2); err == nil {
								a.CaptureRecallHits = hits
							}
						}
					})

				if len(a.CaptureRecallHits) > 0 && strings.TrimSpace(a.CaptureText) != "" {
					hit := a.CaptureRecallHits[0]
					ui.Row(c).FillWidth().Padding(10, 12).Gap(10).AlignItems(ui.Center).Radius(12).
						Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
						IdeaIconBadge(c, domain.ResolveIdeaIconKey(hit.Node), hit.Node.Theme, IconBadgeProps{
							Size:     26,
							IconSize: 14,
							Radius:   7,
						})
						ui.Textf(c, "之前记过：《%s》", hit.Node.Title).
							FontSize(12.5).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(1)
						if PillAction(c, "capture_merge_hit", IconCheck, "合并进去", false).Clicked() {
							a.submitQuickCapture(hit.Node.ID, false)
						}
					})
				}

				ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					if PillAction(c, "capture_scene_chip", IconMapPin, "记下当时场景", a.CaptureShowScene).Clicked() {
						a.CaptureShowScene = !a.CaptureShowScene
					}
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconSpark).Size(14, 14).TextColor(pal.InkSecondary)
						ui.Text(c, "AI 自动整理").FontSize(12.5).TextColor(pal.InkSecondary)
						ui.Switch(c, &a.CaptureAutoOrg).Label("AI 自动整理")
					})
				})

				if a.CaptureShowScene {
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						ui.TextInput(c, &a.CaptureSceneLoc).Placeholder("地点，如：花店门口").FillWidth()
						ui.Row(c).FillWidth().Gap(8).Children(func() {
							ui.TextInput(c, &a.CaptureSceneMood).Placeholder("心情或天气").Grow(1).MinWidth(0)
							ui.TextInput(c, &a.CaptureSceneMusic).Placeholder("在听的歌").Grow(1).MinWidth(0)
						})
					})
				}

				submitBtn := ui.ButtonBase(c).Key("capture_submit").
					FillWidth().Padding(12, 16).Radius(12).Center().
					Background(pal.Ink).TextColor(pal.Card).
					Cursor(ui.CursorPointer).Label("记下来")
				mobile.Ink(submitBtn)
				submitBtn.OnClick(func() {
					a.submitQuickCapture("", a.CaptureAutoOrg)
				}).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconCheck).Size(15, 15).TextColor(pal.Card)
						ui.Text(c, "记下来").FontSize(14).Bold().TextColor(pal.Card).SingleLine()
					})
				})
			})
		})
	})
}

func (a *App) renderCaptureModalityTabs(c *ui.Context, pal SurfacePalette) {
	modes := []struct {
		icon  *ui.SVG
		label string
	}{
		{IconMic, "语音"},
		{IconQuill, "文字"},
		{IconImage, "图片"},
	}

	seg := ui.SegmentedBase(c, &a.CaptureModalityIdx, len(modes))
	seg.Track.FillWidth().Padding(4).Gap(4).Radius(999).Background(pal.SoftPanel).
		Label("记录方式").Children(func() {
		for i, m := range modes {
			active := i == a.CaptureModalityIdx
			s := seg.Segment(i).Padding(7, 12).Radius(999).Gap(6).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label(m.label)
			if active {
				s.Background(pal.Card).Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.08))
			}
			mobile.Ink(s)
			fg := pal.InkSecondary
			if active {
				fg = pal.Ink
			}
			s.Children(func() {
				ui.Icon(c, m.icon).Size(14, 14).TextColor(fg)
				ui.Text(c, m.label).FontSize(13).Bold().TextColor(fg).SingleLine()
			})
		}
	}).FillEqually()
}

func (a *App) renderVoiceCaptureBar(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)
	recording := a.Deps.Media != nil && a.Deps.Media.IsRecording()

	energy := float32(0.42)
	btnLabel := "开始录音"
	caption := "点一下开始说话"
	pulseColor := ColorSuccessInk
	btnBg := pal.Ink
	centerIcon := IconMic
	if recording {
		energy = 1.0
		btnLabel = "结束录音"
		caption = "正在听你说，点一下结束"
		pulseColor = ColorDangerInk
		btnBg = ColorDangerInk
		centerIcon = IconStop
	}

	ui.Column(c).FillWidth().Padding(14, 12).Gap(8).Center().Radius(18).
		Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
		ui.Box(c).Size(156, 142).Center().
			Draw(func(p *ui.Painter, r ui.Rect) {
				DrawVoicePulse(p, r, phase, energy, pulseColor)
			}).
			Children(func() {
				micBtn := ui.ButtonBase(c).Key("voice_centerpiece_btn").Size(84, 84).Radius(42).
					Center().Background(btnBg).
					Shadow(0, 8, 20, 0, ui.RGBA(20, 21, 25, 0.18)).
					Cursor(ui.CursorPointer).Label(btnLabel)
				mobile.Ink(micBtn)
				micBtn.Children(func() {
					ui.Icon(c, centerIcon).Size(24, 24).TextColor(pal.Card)
				})

				if micBtn.Clicked() {
					a.toggleVoiceRecording()
				}
			})

		ui.Text(c, caption).FontSize(12.5).Bold().TextColor(pal.InkSecondary)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Center().Children(func() {
			if PillAction(c, "pick_audio_btn", IconArchive, "导入录音", false).Clicked() {
				a.pickFileAsync(true)
			}
			if a.CaptureAudioPath != "" {
				ui.Row(c).Gap(4).Padding(4, 8).Radius(999).Background(ColorSuccessSoft).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconCheck).Size(12, 12).TextColor(ColorSuccessInk)
					ui.Text(c, "已添加录音").FontSize(11.5).Bold().TextColor(ColorSuccessInk)
					if GhostIconAction(c, "clear_audio_btn", IconClose, "清除录音", false).
						Size(20, 20).Clicked() {
						a.CaptureAudioPath = ""
					}
				})
			}
		})
	})
}

func (a *App) toggleVoiceRecording() {
	if a.Deps.Media == nil {
		a.setErrorBanner("录音功能暂不可用")
		return
	}
	if a.Deps.Media.IsRecording() {
		path, durMs, err := a.Deps.Media.StopRecording()
		if err != nil {
			a.setErrorBanner(err.Error())
			return
		}
		mygo.Haptics.Success()
		a.CaptureAudioPath = path
		a.setBanner(fmt.Sprintf("已录 %.1f 秒，保存后会自动转成文字", float64(durMs)/1000.0))
		return
	}

	mygo.Haptics.Medium()
	path, err := a.Deps.Media.StartRecording()
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	a.CaptureAudioPath = path
	a.setBanner("正在录音，说完点一下即可结束")
}

func (a *App) renderImageCaptureBar(c *ui.Context, pal SurfacePalette) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if PillAction(c, "capture_camera", IconCamera, "拍照", true).Clicked() {
				a.captureCameraAsync()
			}
			if PillAction(c, "capture_gallery", IconImage, "相册选图", false).Clicked() {
				a.pickFileAsync(false)
			}
		})

		if a.CaptureImagePath != "" {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(10).Radius(12).AlignItems(ui.Center).
				Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
				if src := a.loadCachedImage(a.CaptureImagePath); src != nil {
					ui.Image(c, src).Size(44, 44).Radius(8).Fit(ui.Cover)
				} else {
					ui.Box(c).Size(44, 44).Radius(8).Center().Background(pal.Card).Border(1, pal.Border).Children(func() {
						ui.Icon(c, IconImage).Size(18, 18).TextColor(pal.InkSecondary)
					})
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "已选好图片").FontSize(12).Bold().TextColor(ColorSuccessInk)
					ui.Text(c, filepath.Base(a.CaptureImagePath)).FontSize(11).TextColor(pal.InkMuted).SingleLine()
				})
				if GhostIconAction(c, "clear_image_btn", IconClose, "移除图片", false).Size(26, 26).Clicked() {
					a.CaptureImagePath = ""
				}
			})
		}
	})
}

func (a *App) captureCameraAsync() {
	go func() {
		if runtime.GOOS != "android" && runtime.GOOS != "ios" {
			a.Dispatch(func() {
				a.setBanner("电脑上请点「相册选图」选择本地图片，手机上可直接拍照")
			})
			return
		}
		path, err := imagepicker.Capture(context.Background())
		a.Dispatch(func() {
			if err != nil {
				a.setErrorBanner("打开相机失败：" + err.Error())
				return
			}
			if path != "" {
				a.CaptureImagePath = path
			}
		})
	}()
}

func (a *App) pickFileAsync(isAudio bool) {
	go func() {
		if !isAudio && (runtime.GOOS == "android" || runtime.GOOS == "ios") {
			paths, err := imagepicker.Pick(context.Background(), imagepicker.Options{
				Kind:  imagepicker.Images,
				Limit: 1,
			})
			if err != nil {
				a.Dispatch(func() { a.setErrorBanner("打开相册失败：" + err.Error()) })
				return
			}
			if len(paths) > 0 {
				a.Dispatch(func() { a.CaptureImagePath = paths[0] })
			}
			return
		}

		title := "选择图片"
		filters := []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}
		if isAudio {
			title = "选择录音文件"
			filters = []mygo.FileFilter{{Name: "Audio", Extensions: []string{"wav", "ogg", "opus"}}}
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  a.MainWin,
			Title:   title,
			Filters: filters,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.Dispatch(func() {
			if isAudio {
				a.CaptureAudioPath = paths[0]
			} else {
				a.CaptureImagePath = paths[0]
			}
		})
	}()
}

func (a *App) submitQuickCapture(targetNodeID string, autoOrganize bool) {
	modality := "text"
	switch a.CaptureModalityIdx {
	case 0:
		modality = "voice"
	case 2:
		modality = "image"
	}

	in := service.QuickCaptureInput{
		RawText:      a.CaptureText,
		Modality:     modality,
		AudioSrcPath: a.CaptureAudioPath,
		ImageSrcPath: a.CaptureImagePath,
		TargetNodeID: targetNodeID,
		AutoOrganize: autoOrganize,
		Scene: domain.SceneContext{
			Location:    a.CaptureSceneLoc,
			MoodWeather: a.CaptureSceneMood,
			NowPlaying:  a.CaptureSceneMusic,
		},
	}

	if strings.TrimSpace(in.RawText) == "" && in.AudioSrcPath == "" && in.ImageSrcPath == "" {
		a.setErrorBanner("先说句话、写几个字，或者选张图吧")
		return
	}

	a.CaptureModalOpen = false
	a.CaptureText = ""
	a.CaptureAudioPath = ""
	a.CaptureImagePath = ""
	a.CaptureRecallHits = nil

	localIn := in
	localIn.AutoOrganize = false
	out, err := a.Deps.Capture.CaptureQuick(context.Background(), localIn)
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	mygo.Haptics.Success()
	_ = a.ReloadVault()
	a.SelectNode(out.Node.ID)

	if !autoOrganize {
		if out.AIWarning != "" {
			a.setBanner(fmt.Sprintf("已记下：《%s》（%s）", out.Node.Title, out.AIWarning))
		} else {
			a.setBanner(fmt.Sprintf("已记下：《%s》", out.Node.Title))
		}
		return
	}

	if a.Deps.AI == nil || !a.Deps.AI.Configured() {
		a.setBanner(fmt.Sprintf("已记下：《%s》（填好 DeepSeek Key 后可自动整理）", out.Node.Title))
		return
	}

	ev := out.Evidence
	nodeID := out.Node.ID
	a.RunAsync("AI 正在帮你整理这条想法…", func(ctx context.Context) error {
		organized, mergedInto, err := a.Deps.Capture.OrganizeEvidenceIntoCatalog(ctx, ev, nodeID)
		if err != nil {
			return err
		}
		a.Dispatch(func() {
			a.SelectNode(organized.ID)
			if mergedInto != "" {
				a.setBanner(fmt.Sprintf("已合并到《%s》，当时的原记录也留好了", organized.Title))
			} else {
				a.setBanner(fmt.Sprintf("已整理好：《%s》", organized.Title))
			}
		})
		return nil
	})
}
