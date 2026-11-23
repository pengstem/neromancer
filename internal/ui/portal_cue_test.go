package ui

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPortalCueCyclePhasesAndContinuity(t *testing.T) {
	// Stage 0: Downward Swipe Arrow wave
	fArrow := evalPortalCueFrame(1.0)
	if fArrow.ArrowAlpha < 0.99 || fArrow.TextAlpha != 0 || fArrow.IconAlpha != 0 || !fArrow.Animating {
		t.Fatalf("expected active arrow stage at t=1.0s, got %+v", fArrow)
	}

	// Verify downward wave motion bounds & non-trivial variation across one wave period
	minUY, maxUY := float32(99), float32(-99)
	minLY, maxLY := float32(99), float32(-99)
	for step := 0; step <= 36; step++ {
		ts := float32(step) * 0.05 // 0.00 .. 1.80s
		fw := evalPortalCueFrame(ts)
		if fw.UpperChevY < -2.5 || fw.UpperChevY > 4.0 {
			t.Fatalf("UpperChevY out of subtle bounds at t=%v: %v", ts, fw.UpperChevY)
		}
		if fw.LowerChevY < -2.5 || fw.LowerChevY > 4.0 {
			t.Fatalf("LowerChevY out of subtle bounds at t=%v: %v", ts, fw.LowerChevY)
		}
		minUY = min(minUY, fw.UpperChevY)
		maxUY = max(maxUY, fw.UpperChevY)
		minLY = min(minLY, fw.LowerChevY)
		maxLY = max(maxLY, fw.LowerChevY)
	}
	if maxUY-minUY < 2.0 || maxLY-minLY < 2.0 {
		t.Fatalf("expected downward swipe chevron wave motion, got upper=[%v,%v] lower=[%v,%v]", minUY, maxUY, minLY, maxLY)
	}

	// Arrow -> Text crossfade
	fCross1 := evalPortalCueFrame(3.95)
	if fCross1.ArrowAlpha <= 0 || fCross1.TextAlpha <= 0 || !fCross1.Animating {
		t.Fatalf("expected Arrow->Text crossfade at t=3.95s, got %+v", fCross1)
	}

	// Text calm static hold
	fText := evalPortalCueFrame(5.5)
	if fText.TextAlpha < 0.99 || fText.ArrowAlpha != 0 || fText.IconAlpha != 0 || fText.Animating || fText.NextWakeSec <= 0 {
		t.Fatalf("expected calm Text hold at t=5.5s, got %+v", fText)
	}

	// Text -> Icon crossfade
	fCross2 := evalPortalCueFrame(7.15)
	if fCross2.TextAlpha <= 0 || fCross2.IconAlpha <= 0 || !fCross2.Animating {
		t.Fatalf("expected Text->Icon crossfade at t=7.15s, got %+v", fCross2)
	}

	// Icon calm static hold
	fIcon := evalPortalCueFrame(8.8)
	if fIcon.IconAlpha < 0.99 || fIcon.ArrowAlpha != 0 || fIcon.TextAlpha != 0 || fIcon.Animating || fIcon.NextWakeSec <= 0 {
		t.Fatalf("expected calm Icon hold at t=8.8s, got %+v", fIcon)
	}

	// Icon -> Arrow crossfade
	fCross3 := evalPortalCueFrame(10.5)
	if fCross3.IconAlpha <= 0 || fCross3.ArrowAlpha <= 0 || !fCross3.Animating {
		t.Fatalf("expected Icon->Arrow crossfade at t=10.5s, got %+v", fCross3)
	}

	// Seamless wrap continuity across 10.80s -> 0.0s
	fEnd := evalPortalCueFrame(portalCueCycleSec - 0.0001)
	fStart := evalPortalCueFrame(0.0)
	if math.Abs(float64(fEnd.ArrowAlpha-fStart.ArrowAlpha)) > 0.01 ||
		math.Abs(float64(fEnd.ArrowY-fStart.ArrowY)) > 0.02 ||
		math.Abs(float64(fEnd.ArrowScale-fStart.ArrowScale)) > 0.01 ||
		math.Abs(float64(fEnd.IconAlpha-fStart.IconAlpha)) > 0.01 {
		t.Fatalf("wrap discontinuity across cycle boundary: end=%+v start=%+v", fEnd, fStart)
	}
}

func TestPortalCueInteractiveAndRenderStages(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := ui.NewTesterWith(app.RootView, 412, 892, ui.TesterOptions{
		Preferences: ui.Preferences{ReduceMotion: false},
	})
	tt.Frame()

	portalRect, ok := tt.Find("进入思维宫殿")
	if !ok {
		t.Fatalf("expected accessible top-left portal button \"进入思维宫殿\"")
	}
	if portalRect.Y > 80 {
		t.Fatalf("expected portal near top edge (Y <= 80), got %+v", portalRect)
	}

	auditDir := resolveVisualAuditDir()

	// Stage 0: Arrow swipe peak (~0.90s into cycle)
	tt.Advance(900 * time.Millisecond)
	tt.Frame()
	maybeSaveAuditPNG(t, auditDir, tt, "portal_cue_arrow.png")

	// Stage 1: Calm "思维宫殿" text (~5.50s into cycle)
	tt.Advance(4600 * time.Millisecond)
	tt.Frame()
	maybeSaveAuditPNG(t, auditDir, tt, "portal_cue_text.png")

	// Stage 2: Calm Mind Palace constellation icon (~8.80s into cycle)
	tt.Advance(3300 * time.Millisecond)
	tt.Frame()
	maybeSaveAuditPNG(t, auditDir, tt, "portal_cue_icon.png")

	// Click must open Mind Palace cleanly at any stage
	if err := tt.Click("进入思维宫殿"); err != nil {
		t.Fatalf("click top-left portal cue: %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("expected SpaceArchive after clicking portal cue, got %v", app.ActiveSpace)
	}
}
