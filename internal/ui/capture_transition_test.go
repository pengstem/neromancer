package ui

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestCaptureRemovesAutoOrgToggleAndRunsAIDefault(t *testing.T) {
	var aiCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aiCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"finish_reason": "stop",
				"message": {
					"content": "{\"action\":\"create\",\"folder\":\"器物与光影美学\",\"title\":\"街角暖黄玻璃台灯\",\"conversational_summary\":\"记下了街角橱窗里那盏暖黄玻璃台灯的柔和光晕。\",\"tags\":[\"台灯\",\"光影\"],\"theme\":\"amber\",\"weight\":0.82,\"visual\":{\"motif_type\":\"spark\",\"caption\":\"暖黄光晕\",\"energy\":0.85},\"guiding_questions\":[\"想放在书桌左边还是窗台？\"]}"
				}
			}]
		}`))
	}))
	defer srv.Close()

	app, cleanup := newAuditTestApp(t)
	defer cleanup()
	app.Deps.AI.UpdateCredentials(srv.URL, "sk-test-auto-default", "deepseek-chat")

	tt := newFastTester(app.RootView, 412, 892)
	settleAudit(tt)

	// Verify none of the 3 capture modes render an "AI 自动整理" button or switch.
	for _, mod := range []string{"语音", "文字", "图文"} {
		if err := tt.Click("记一笔"); err != nil {
			t.Fatalf("open dial for %s: %v", mod, err)
		}
		settleAudit(tt)
		if err := tt.Click(mod); err != nil {
			t.Fatalf("select %s: %v", mod, err)
		}
		settleAudit(tt)

		if tt.HasText("AI 自动整理") {
			t.Fatalf("expected 'AI 自动整理' toggle to be removed in %s mode, texts=%v", mod, tt.Texts())
		}
		if _, found := tt.Find("AI 自动整理"); found {
			t.Fatalf("expected no 'AI 自动整理' control in accessibility tree for %s", mod)
		}

		if err := tt.Click("收起"); err != nil {
			t.Fatalf("close %s mode: %v", mod, err)
		}
		settleAudit(tt)
	}

	// Open Text capture, enter a thought, and click "记下来": AI auto-organization must run automatically!
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("open dial: %v", err)
	}
	settleAudit(tt)
	if err := tt.Click("文字"); err != nil {
		t.Fatalf("enter text capture: %v", err)
	}
	settleAudit(tt)

	app.CaptureText = "街角橱窗里有一盏暖黄色的复古玻璃台灯"
	tt.Frame()
	if err := tt.Click("记下来"); err != nil {
		t.Fatalf("submit capture: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		app.FlushPending()
		tt.Frame()
		if aiCalls.Load() >= 1 && app.BusyTask == "" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	if aiCalls.Load() != 1 {
		t.Fatalf("expected AI auto-organize to fire automatically once without any toggle, got %d calls", aiCalls.Load())
	}
	foundOrganized := false
	for _, n := range app.Nodes {
		if n.Title == "街角暖黄玻璃台灯" {
			foundOrganized = true
			break
		}
	}
	if !foundOrganized {
		t.Fatalf("expected organized node '街角暖黄玻璃台灯' in vault, got %d nodes", len(app.Nodes))
	}
}

func TestCaptureThreeModesEnterDedicatedFormatsWithoutSwitcher(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := ui.NewTester(app.RootView, 412, 892)
	settleAudit(tt)

	cases := []struct {
		dialLabel   string
		wantTitle   string
		wantUnique  string
		forbidTexts []string
	}{
		{
			dialLabel:   "语音",
			wantTitle:   "语音记录",
			wantUnique:  "点一下开始说话",
			forbidTexts: []string{"文字记录", "图文记录", "拍下眼前的画面，或从相册选一张图", "记录方式", "文字", "图文", "图片"},
		},
		{
			dialLabel:   "文字",
			wantTitle:   "文字记录",
			wantUnique:  "",
			forbidTexts: []string{"语音记录", "图文记录", "点一下开始说话", "导入录音", "拍照", "相册选图", "记录方式", "语音", "图文", "图片"},
		},
		{
			dialLabel:   "图文",
			wantTitle:   "图文记录",
			wantUnique:  "拍下眼前的画面，或从相册选一张图",
			forbidTexts: []string{"语音记录", "文字记录", "点一下开始说话", "导入录音", "记录方式", "语音", "文字"},
		},
	}

	for _, tc := range cases {
		if err := tt.Click("记一笔"); err != nil {
			t.Fatalf("open dial for %s: %v", tc.dialLabel, err)
		}
		settleAudit(tt)
		if err := tt.Click(tc.dialLabel); err != nil {
			t.Fatalf("click %s: %v", tc.dialLabel, err)
		}
		tt.Advance(120 * time.Millisecond)
		tt.Frame()
		if app.CaptureAnim.Phase != CaptureAnimOpening {
			t.Fatalf("expected %s to be opening at 120 ms, got %v", tc.dialLabel, app.CaptureAnim.Phase)
		}
		if tt.HasText("记下来") || tt.HasText("收起") {
			t.Fatalf("%s built live editor controls during the opening animation", tc.dialLabel)
		}
		settleAudit(tt)

		if !tt.HasText(tc.wantTitle) || (tc.wantUnique != "" && !tt.HasText(tc.wantUnique)) {
			t.Fatalf("%s mode missing dedicated UI (%q / %q), texts=%v", tc.dialLabel, tc.wantTitle, tc.wantUnique, tt.Texts())
		}
		if _, hasSeg := tt.Find("记录方式"); hasSeg {
			t.Fatalf("expected modality switcher '记录方式' to be removed in %s mode", tc.dialLabel)
		}
		for _, bad := range tc.forbidTexts {
			if tt.HasText(bad) {
				t.Fatalf("%s dedicated mode leaked forbidden switcher/other-mode label %q, texts=%v", tc.dialLabel, bad, tt.Texts())
			}
		}

		if err := tt.Click("收起"); err != nil {
			t.Fatalf("close %s mode: %v", tc.dialLabel, err)
		}
		settleAudit(tt)
	}
}

func TestCaptureSmoothLocalToFullscreenAndShrinkIntoTopMindPalace(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	// Real motion enabled (ReduceMotion: false) to verify both container-transform animations frame-by-frame.
	tt := ui.NewTester(app.RootView, 412, 892)
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	tt.Frame()

	// 1. Open homepage speed-dial and let the 3 circular buttons settle
	if err := tt.Click("记一笔"); err != nil {
		t.Fatalf("click 记一笔: %v", err)
	}
	for i := 0; i < 8; i++ {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}

	txtOrbRect, ok := tt.Find("文字")
	if !ok {
		t.Fatalf("missing 文字 dial button")
	}
	if txtOrbRect.X < 300 || txtOrbRect.Y < 600 {
		t.Fatalf("expected 文字 dial button near bottom-right, got %+v", txtOrbRect)
	}

	// 2. Click "文字": verify smooth Local -> Fullscreen opening animation
	if err := tt.Click("文字"); err != nil {
		t.Fatalf("click 文字 dial button: %v", err)
	}
	tt.Advance(45 * time.Millisecond)
	tt.Frame()
	if app.CaptureAnim.Phase != CaptureAnimOpening {
		t.Fatalf("expected CaptureAnimOpening at 45ms, got %v", app.CaptureAnim.Phase)
	}
	// Advance beyond the opening duration and verify the editor is mounted.
	tt.Advance(captureOpenDur)
	tt.Frame()
	if app.CaptureAnim.Phase != CaptureAnimOpen {
		t.Fatalf("expected CaptureAnimOpen after spring settles, got %v", app.CaptureAnim.Phase)
	}

	// Verify full-screen layout: header near top (Y < 80) and "记下来" button near bottom (Y > 800)
	closeR, okClose := tt.Find("收起")
	submitR, okSub := tt.Find("记下来")
	if !okClose || !okSub {
		t.Fatalf("missing full-screen controls: close=%v submit=%v", okClose, okSub)
	}
	if closeR.Y > 80 || submitR.Y < 790 {
		t.Fatalf("expected full-screen capture spanning top-to-bottom of 412x892, got closeY=%.1f submitY=%.1f", closeR.Y, submitR.Y)
	}

	// 3. Submit capture: verify Beat 1 ("慢慢全屏切回去" -> floating capsule) & Beat 2 ("缩进上方" -> Mind Palace ceiling)!
	app.CaptureText = "把傍晚火烧云的渐变存成一组配色卡"
	tt.Frame()
	if err := tt.Click("记下来"); err != nil {
		t.Fatalf("click 记下来: %v", err)
	}

	tt.Advance(20 * time.Millisecond)
	tt.Frame()
	if app.CaptureModalOpen || app.CaptureAnim.Phase != CaptureAnimClosingPalace {
		t.Fatalf("expected CaptureModalOpen=false & CaptureAnimClosingPalace right after submit, got open=%v phase=%v",
			app.CaptureModalOpen, app.CaptureAnim.Phase)
	}

	// Beat 1 end (~350ms): full-screen has condensed back into the centered Idea Capsule over the Homepage!
	const capsuleLabel = "《把傍晚火烧云的渐变存成一组配色卡》"
	tt.Advance(330 * time.Millisecond) // t = 350ms (u ≈ 0.46)
	tt.Frame()
	if !tt.HasText(capsuleLabel) || !tt.HasText("AI 灵感画像") {
		t.Fatalf("expected condensed Idea Capsule floating above visible Homepage at Beat 1, texts=%v", tt.Texts())
	}
	capMidR, okCapMid := tt.Find(capsuleLabel)
	if !okCapMid {
		t.Fatalf("missing floating capsule text at 350ms")
	}

	// Beat 2 mid (~530ms): capsule glides upward toward the Mind Palace!
	tt.Advance(180 * time.Millisecond) // t = 530ms (u ≈ 0.70)
	tt.Frame()
	capUpperR, okCapUpper := tt.Find(capsuleLabel)
	if !okCapUpper {
		t.Fatalf("missing floating capsule text at 530ms")
	}

	// Beat 2 near top (~700ms): capsule reaches the top Mind Palace header & triggers portal pulse!
	tt.Advance(170 * time.Millisecond) // t = 700ms (u ≈ 0.90)
	tt.Frame()
	capTopR, okCapTop := tt.Find(capsuleLabel)
	if !okCapTop {
		t.Fatalf("missing floating capsule text at 700ms")
	}

	if !(capMidR.Y > capUpperR.Y && capUpperR.Y > capTopR.Y && capTopR.Y < 90) {
		t.Fatalf("expected Idea Capsule to glide monotonically upward into top Mind Palace: midY=%.1f upperY=%.1f topY=%.1f",
			capMidR.Y, capUpperR.Y, capTopR.Y)
	}
	if app.CaptureAnim.PortalPulseAt.IsZero() {
		t.Fatalf("expected top Mind Palace portal reception pulse to trigger as capsule tucks into top")
	}

	// Settle remaining palace tuck + portal reception glow and verify zero idle frame loop
	for i := 0; i < 16; i++ {
		tt.Advance(60 * time.Millisecond)
		tt.Frame()
	}
	if app.CaptureAnim.Phase != CaptureAnimIdle {
		t.Fatalf("expected CaptureAnimIdle after completion, got %v", app.CaptureAnim.Phase)
	}
	if tt.FramePending() {
		t.Fatalf("expected zero continuous idle animation frames once palace reception glow finishes")
	}
}
