package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/pengstem/neromancer/internal/service"
)

func (a *App) submitFloatingCapture(audioPath string) {
	text := strings.TrimSpace(a.FloatQuickText)
	if text == "" && audioPath == "" {
		return
	}
	a.FloatQuickText = ""
	a.FloatExpanded = false

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
