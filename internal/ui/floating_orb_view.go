package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/service"
)

// FloatingOrbView renders the frameless Always-On-Top desktop quick-capture companion window.
func (a *App) FloatingOrbView(c *ui.Context) {
	a.FlushPending()
	pal := a.Palette()

	ui.Themed(c, NeromancerTheme()).Fill().Children(func() {
		if !a.FloatExpanded {
			ui.Column(c).Fill().Center().Padding(8).
				Background(ui.Transparent).
				Children(func() {
					ui.ButtonBase(c).Size(68, 68).Radius(34).DragWindow().
						Center().
						Background(pal.Ink).
						Border(1, ui.RGBA(255, 255, 255, 0.16)).
						Shadow(0, 8, 20, 0, ui.RGBA(20, 21, 25, 0.25)).
						Tooltip("拖动移位 · 点一下随手记").
						Label("随手记").
						Cursor(ui.CursorPointer).
						Children(func() {
							ui.Icon(c, IconSpark).Size(24, 24).TextColor(pal.Card)
						}).
						OnClick(func() {
							a.FloatExpanded = true
							if a.FloatWin != nil {
								a.FloatWin.SetSize(360, 250)
							}
						})
				})
			return
		}

		ui.Column(c).Fill().Padding(14).Gap(10).Radius(18).
			Background(pal.Card).Border(1, pal.Border).
			Children(func() {
				ui.Row(c).FillWidth().Gap(7).AlignItems(ui.Center).DragWindow().Children(func() {
					ui.Icon(c, IconSpark).Size(15, 15).TextColor(pal.Ink).Shrink(0)
					ui.Text(c, "随手记").FontSize(13).Bold().TextColor(pal.Ink).Grow(1)
					if PillAction(c, "float_main_win", IconArchiveFolder, "主界面", false).Clicked() && a.MainWin != nil {
						a.MainWin.Show()
						a.MainWin.Focus()
					}
					if GhostIconAction(c, "float_collapse", IconChevronDown, "收起", false).Clicked() {
						a.FloatExpanded = false
						if a.FloatWin != nil {
							a.FloatWin.SetSize(96, 96)
						}
					}
				})

				ui.Box(c).FillWidth().Grow(1).Padding(10, 12).Radius(12).
					Background(pal.SoftPanel).Border(1, pal.Border).
					Children(func() {
						ui.TextAreaBase(c, &a.FloatQuickText).
							Placeholder("想到什么直接记下…").
							PlaceholderColor(pal.InkMuted).
							TextColor(pal.Ink).
							Label("随手记输入").
							Fill().AutoFocus()
					})

				ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					if a.Deps.Media != nil && a.Deps.Media.IsRecording() {
						if PillAction(c, "float_stop_rec", IconStop, "结束", true).Clicked() {
							path, _, err := a.Deps.Media.StopRecording()
							if err == nil {
								a.submitFloatingCapture(path)
							} else {
								a.setErrorBanner(err.Error())
							}
						}
					} else {
						if PillAction(c, "float_start_rec", IconMic, "语音", false).Clicked() && a.Deps.Media != nil {
							if _, err := a.Deps.Media.StartRecording(); err != nil {
								a.setErrorBanner(err.Error())
							}
						}
					}

					ui.Spacer(c).Grow(1)

					if PillAction(c, "float_submit", IconCheck, "记下来", true).Clicked() {
						a.submitFloatingCapture("")
					}
				})
			})
	})
}

func (a *App) submitFloatingCapture(audioPath string) {
	text := strings.TrimSpace(a.FloatQuickText)
	if text == "" && audioPath == "" {
		return
	}
	a.FloatQuickText = ""
	a.FloatExpanded = false
	if a.FloatWin != nil {
		a.FloatWin.SetSize(96, 96)
	}

	modality := "text"
	if audioPath != "" {
		modality = "voice"
	}

	localOut, err := a.Deps.Capture.CaptureQuick(context.Background(), service.QuickCaptureInput{
		RawText:      text,
		Modality:     modality,
		AudioSrcPath: audioPath,
		AutoOrganize: false,
	})
	if err != nil {
		a.Dispatch(func() { a.setErrorBanner(err.Error()) })
		return
	}
	a.Dispatch(func() {
		_ = a.ReloadVault()
		a.SelectNode(localOut.Node.ID)
		a.TriggerGlobeCaptureRipple(localOut.Node)
		a.setBanner(fmt.Sprintf("已记下：《%s》", localOut.Node.Title))
	})

	if a.Deps.AI != nil && a.Deps.AI.Configured() {
		ev := localOut.Evidence
		nodeID := localOut.Node.ID
		a.RunAsync("AI 正在帮你整理…", func(ctx context.Context) error {
			organized, _, err := a.Deps.Capture.OrganizeEvidenceIntoCatalog(ctx, ev, nodeID)
			if err != nil {
				return err
			}
			a.Dispatch(func() {
				a.SelectNode(organized.ID)
				a.TriggerGlobeCaptureRipple(organized)
				a.setBanner(fmt.Sprintf("已整理好：《%s》", organized.Title))
			})
			return nil
		})
	}
}
