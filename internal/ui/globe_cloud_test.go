package ui

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestGlobeMathAndFibonacciSphere(t *testing.T) {
	pts := fibonacciSpherePoints(16)
	if len(pts) != 16 {
		t.Fatalf("expected 16 points, got %d", len(pts))
	}
	if pts[0] != ([3]float64{0, 0, 1}) {
		t.Fatalf("expected index 0 at (0,0,1), got %+v", pts[0])
	}
	for i, p := range pts {
		norm := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
		if math.Abs(norm-1.0) > 1e-9 {
			t.Fatalf("point[%d] not unit length: %v", i, norm)
		}
		for j := i + 1; j < len(pts); j++ {
			dx := p[0] - pts[j][0]
			dy := p[1] - pts[j][1]
			dz := p[2] - pts[j][2]
			if math.Sqrt(dx*dx+dy*dy+dz*dz) < 0.05 {
				t.Fatalf("points %d and %d too close: %+v vs %+v", i, j, p, pts[j])
			}
		}
	}

	rot := mat3Orthonormalize(mat3Mul(mat3RotX(0.42), mat3RotY(-0.78)))
	checkOrtho(t, "rot", rot)

	// Focus alignment accuracy across all sphere points (including antipodal test).
	for i, pt := range pts {
		target := mat3FocusTarget(pt, rot)
		checkOrtho(t, "focus_target", target)
		aligned := mat3Vec(target, pt)
		if math.Abs(aligned[0]) > 1e-6 || math.Abs(aligned[1]) > 1e-6 || math.Abs(aligned[2]-1.0) > 1e-6 {
			t.Fatalf("mat3FocusTarget[%d] failed to align to (0,0,1): got %+v", i, aligned)
		}
	}

	// Antipodal case: point facing (0, 0, -1).
	flipX := mat3RotY(math.Pi)
	anti := mat3FocusTarget([3]float64{0, 0, 1}, flipX)
	checkOrtho(t, "antipodal", anti)
	alignedAnti := mat3Vec(anti, [3]float64{0, 0, 1})
	if math.Abs(alignedAnti[0]) > 1e-6 || math.Abs(alignedAnti[1]) > 1e-6 || math.Abs(alignedAnti[2]-1.0) > 1e-6 {
		t.Fatalf("antipodal focus failed: got %+v", alignedAnti)
	}

	// SLERP endpoints & midpoint orthonormality.
	m0 := mat3Identity()
	m1 := mat3Mul(mat3RotX(0.9), mat3RotY(1.4))
	s0 := mat3Slerp(m0, m1, 0.0)
	s1 := mat3Slerp(m0, m1, 1.0)
	sMid := mat3Slerp(m0, m1, 0.5)
	checkOrtho(t, "slerp_0", s0)
	checkOrtho(t, "slerp_1", s1)
	checkOrtho(t, "slerp_mid", sMid)
	for i := 0; i < 9; i++ {
		if math.Abs(s0[i]-m0[i]) > 1e-6 {
			t.Fatalf("slerp(0) mismatch at %d: %v vs %v", i, s0[i], m0[i])
		}
		if math.Abs(s1[i]-m1[i]) > 1e-6 {
			t.Fatalf("slerp(1) mismatch at %d: %v vs %v", i, s1[i], m1[i])
		}
	}
}

func checkOrtho(t *testing.T, name string, m [9]float64) {
	t.Helper()
	c0 := [3]float64{m[0], m[3], m[6]}
	c1 := [3]float64{m[1], m[4], m[7]}
	c2 := [3]float64{m[2], m[5], m[8]}
	if math.Abs(vec3Dot(c0, c0)-1) > 1e-6 || math.Abs(vec3Dot(c1, c1)-1) > 1e-6 || math.Abs(vec3Dot(c2, c2)-1) > 1e-6 {
		t.Fatalf("%s columns not unit length: %+v", name, m)
	}
	if math.Abs(vec3Dot(c0, c1)) > 1e-6 || math.Abs(vec3Dot(c0, c2)) > 1e-6 || math.Abs(vec3Dot(c1, c2)) > 1e-6 {
		t.Fatalf("%s columns not orthogonal: %+v", name, m)
	}
	det := vec3Dot(c0, vec3Cross(c1, c2))
	if math.Abs(det-1.0) > 1e-6 {
		t.Fatalf("%s det != +1: %v", name, det)
	}
}

func TestDashboardWordCloudGlobeInteractions(t *testing.T) {
	app, cleanup := newAuditTestApp(t)
	defer cleanup()

	tt := ui.NewTester(app.RootView, 412, 892)
	settleAudit(tt)

	terms := app.DashboardDigest.WordCloud
	if len(terms) == 0 {
		t.Fatalf("expected non-empty WordCloud terms")
	}
	if !tt.HasText("灵感星云球") || !tt.HasText(terms[0].Word) {
		t.Fatalf("dashboard globe missing expected title or top word %q, texts=%v", terms[0].Word, tt.Texts())
	}
	assertAuditNoEmojis(t, "globe_initial", tt.Texts())

	// 1. Touch drag inside the circular globe rotates Globe.Rot and locks DashboardScroll.Y.
	centerRect, ok := tt.Find(terms[0].Word)
	if !ok {
		t.Fatalf("could not locate top globe word %q", terms[0].Word)
	}
	cx := centerRect.X + centerRect.W*0.5
	cy := centerRect.Y + centerRect.H*0.5
	rotBefore := app.Globe.Rot

	tt.FingerDown(cx, cy)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(cx+45, cy-25)
	tt.Advance(20 * time.Millisecond)
	tt.FingerMove(cx+82, cy-48)
	tt.Advance(20 * time.Millisecond)
	tt.FingerUp(cx+82, cy-48)
	tt.Frame()

	if app.DashboardScroll.Y != 0 {
		t.Fatalf("expected vertical DashboardScroll to remain locked during globe drag, got Y=%v", app.DashboardScroll.Y)
	}
	diff := 0.0
	for i := 0; i < 9; i++ {
		diff += math.Abs(app.Globe.Rot[i] - rotBefore[i])
	}
	if diff < 0.05 {
		t.Fatalf("expected Globe.Rot to rotate after finger drag, diff=%v", diff)
	}

	// Verify inertia coasts and velocity decays smoothly over time.
	v0 := math.Hypot(app.Globe.VelYaw, app.Globe.VelPitch)
	if v0 <= 0.05 {
		t.Fatalf("expected non-trivial post-drag inertia velocity, got %v", v0)
	}
	for i := 0; i < 8; i++ {
		tt.Advance(50 * time.Millisecond)
	}
	v1 := math.Hypot(app.Globe.VelYaw, app.Globe.VelPitch)
	if v1 >= v0 {
		t.Fatalf("expected inertia velocity to decay: before=%v after=%v", v0, v1)
	}
	for i := 0; i < 30; i++ {
		tt.Advance(50 * time.Millisecond)
	}

	// 2. Double-tap blank area inside globe resets orientation so #1 term faces front.
	app.Globe.Rot = mat3Orthonormalize(mat3RotY(1.25))
	app.Globe.VelYaw = 0
	app.Globe.VelPitch = 0
	tt.Frame()

	// Pick a point near bottom-right inside the circular hitzone unlikely to hit a foreground button.
	pts := fibonacciSpherePoints(len(terms))
	blankX, blankY := cx, cy
	for _, cand := range [][2]float32{
		{cx + 55, cy + 55},
		{cx - 55, cy + 55},
		{cx + 55, cy - 55},
		{cx - 55, cy - 55},
		{cx, cy + 68},
	} {
		// Ensure candidate doesn't land on any foreground projected word rect
		clean := true
		for i, t := range terms {
			rv := mat3Vec(app.Globe.Rot, pts[i])
			if rv[2] < 0.05 {
				continue
			}
			if r, found := tt.Find(t.Word); found {
				if cand[0] >= r.X && cand[0] <= r.X+r.W && cand[1] >= r.Y && cand[1] <= r.Y+r.H {
					clean = false
					break
				}
			}
		}
		if clean {
			blankX, blankY = cand[0], cand[1]
			break
		}
	}

	tt.ClickAt(blankX, blankY)
	tt.Advance(90 * time.Millisecond)
	tt.ClickAt(blankX, blankY)
	for i := 0; i < 10; i++ {
		tt.Advance(50 * time.Millisecond)
	}
	if !mat3ApproxIdentity(app.Globe.Rot) {
		t.Fatalf("expected double-tap blank globe to reset rotation near identity, got %+v", app.Globe.Rot)
	}

	// 3. Backface words (z < 0) are PassThrough and ignore taps.
	var backWord string
	var backPos [2]float32
	for i := len(terms) - 1; i >= 0; i-- {
		rv := mat3Vec(app.Globe.Rot, pts[i])
		if rv[2] < -0.25 {
			if r, found := tt.Find(terms[i].Word); found {
				// Confirm no foreground button sits over this center
				px, py := r.X+r.W*0.5, r.Y+r.H*0.5
				covered := false
				for j, ft := range terms {
					if mat3Vec(app.Globe.Rot, pts[j])[2] < 0.05 {
						continue
					}
					if fr, fOK := tt.Find(ft.Word); fOK {
						if px >= fr.X && px <= fr.X+fr.W && py >= fr.Y && py <= fr.Y+fr.H {
							covered = true
							break
						}
					}
				}
				if !covered {
					backWord = terms[i].Word
					backPos = [2]float32{px, py}
					break
				}
			}
		}
	}
	if backWord != "" {
		if err := tt.Click(backWord); err == nil {
			t.Fatalf("expected backface word %q to be non-clickable (PassThrough)", backWord)
		}
		tt.ClickAt(backPos[0], backPos[1])
		tt.Frame()
		if app.Globe.FocusedWord == backWord {
			t.Fatalf("expected backface tap at (%v,%v) NOT to focus %q", backPos[0], backPos[1], backWord)
		}
	}

	// 4. Tap foreground word -> focuses & opens preview card -> "看相关档案" navigates to SpaceArchive.
	var targetTermIdx int = -1
	for i := 1; i < len(terms); i++ {
		rv := mat3Vec(app.Globe.Rot, pts[i])
		if rv[2] > 0.30 {
			if err := tt.Click(terms[i].Word); err == nil {
				targetTermIdx = i
				break
			}
		}
	}
	if targetTermIdx < 0 {
		targetTermIdx = 0
		if err := tt.Click(terms[0].Word); err != nil {
			t.Fatalf("click top foreground word %q: %v", terms[0].Word, err)
		}
	}
	targetTerm := terms[targetTermIdx]
	if app.Globe.FocusedWord != targetTerm.Word {
		t.Fatalf("expected FocusedWord=%q, got %q", targetTerm.Word, app.Globe.FocusedWord)
	}
	for i := 0; i < 10; i++ {
		tt.Advance(50 * time.Millisecond)
	}
	focusedZ := mat3Vec(app.Globe.Rot, pts[targetTermIdx])[2]
	if focusedZ < 0.96 {
		t.Fatalf("expected focused word %q to rotate to front-center (z>0.96), got z=%v", targetTerm.Word, focusedZ)
	}
	if !tt.HasText("看相关档案") {
		t.Fatalf("expected focus preview card with \"看相关档案\" button, texts=%v", tt.Texts())
	}
	assertAuditNoEmojis(t, "globe_focused", tt.Texts())

	if err := tt.Click("看相关档案"); err != nil {
		t.Fatalf("click \"看相关档案\": %v", err)
	}
	settleAudit(tt)
	if app.ActiveSpace != SpaceArchive {
		t.Fatalf("expected SpaceArchive after clicking \"看相关档案\", got %v", app.ActiveSpace)
	}
	if targetTerm.FilterTag != "" && app.ActiveTag != targetTerm.FilterTag {
		t.Fatalf("expected ActiveTag=%q, got %q", targetTerm.FilterTag, app.ActiveTag)
	}
}
