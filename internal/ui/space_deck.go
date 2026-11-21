package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	spaceDeckSpringDur = 360 * time.Millisecond
	spaceScrollSettle  = 160 * time.Millisecond
)

// SpaceDeckState stores frame-persistent vertical two-floor state where the
// Inspiration Archive sits directly above the Home Dashboard (WeChat "second floor" style).
type SpaceDeckState struct {
	Initialized bool
	ViewportH   float32
	Offset      ui.Point // Y in [-ViewportH, 0]; -ViewportH = Dashboard, 0 = Archive
	LastSpace   Space

	CrossedThreshold bool

	// Boundary handoff when an inner Scroll container owns touch panning or wheel/trackpad scrolling.
	ScrollPulling bool
	ScrollFrom    Space
	ScrollVelY    float32
	ScrollLastAt  time.Time
}

func spaceTargetY(sp Space, h float32) float32 {
	if sp == SpaceArchive {
		return 0
	}
	return -h
}

func spacePullThreshold(h float32) float32 {
	return min(float32(124), max(float32(72), h*0.18))
}

func (a *App) renderSpaceDeck(c *ui.Context, pal SurfacePalette) {
	stage := ui.Box(c).Key("space_deck_stage").Grow(1).FillWidth().Clip().
		HandleInput(func(ev ui.InputEvent) bool {
			// Once a boundary scroll-pull is active, the moving track can slide out from under
			// the original touch-down coordinate; keep routing InputScroll/Momentum here.
			if a.SpaceDeck.ScrollPulling {
				return a.handleSpaceBoundaryScroll(ev, a.SpaceDeck.ScrollFrom, false)
			}
			return false
		})
	h := stage.Bounds().H
	if h <= 0 {
		_, h = c.Size()
		if h <= 0 {
			h = 800
		}
	}

	deck := &a.SpaceDeck
	if !deck.Initialized {
		deck.Initialized = true
		deck.ViewportH = h
		deck.LastSpace = a.ActiveSpace
		deck.Offset = ui.Point{Y: spaceTargetY(a.ActiveSpace, h)}
	} else if math.Abs(float64(deck.ViewportH-h)) > 0.5 {
		deck.ViewportH = h
		deck.ScrollPulling = false
		deck.ScrollVelY = 0
		deck.LastSpace = a.ActiveSpace
		deck.Offset = ui.Point{Y: spaceTargetY(a.ActiveSpace, h)}
	} else if deck.LastSpace != a.ActiveSpace {
		// Programmatic transition (button tap, Globe word jump, capture/import flow, tests).
		deck.LastSpace = a.ActiveSpace
		deck.ScrollPulling = false
		deck.ScrollVelY = 0
		deck.Offset = ui.Point{Y: spaceTargetY(a.ActiveSpace, h)}
	}

	now := c.Now()
	if deck.ScrollPulling {
		if !deck.ScrollLastAt.IsZero() && now.Sub(deck.ScrollLastAt) >= spaceScrollSettle {
			a.commitSpaceScrollPull(h)
		} else {
			c.AnimationFrame()
		}
	}

	threshold := spacePullThreshold(h)

	stage.Children(func() {
		track := ui.Column(c).Key("space_deck_track").
			Absolute().Left(0).Right(0).Top(0).Height(h * 2)

		anim := ui.Animation{Spring: ui.SpringOptions{Duration: spaceDeckSpringDur}}
		if deck.ScrollPulling {
			anim = ui.Animation{Duration: time.Millisecond, Ease: ui.Linear}
		}

		startSpace := a.ActiveSpace
		track.Draggable(&deck.Offset, ui.DraggableOptions{
			Axis:      ui.Vertical,
			Bounds:    &ui.Rect{Y: -h, H: h},
			Rigid:     true,
			Animation: anim,
			Snap: func(projected ui.Point) ui.Point {
				if startSpace == SpaceDashboard {
					if projected.Y+h >= threshold {
						return ui.Point{Y: 0}
					}
					return ui.Point{Y: -h}
				}
				if -projected.Y >= threshold {
					return ui.Point{Y: -h}
				}
				return ui.Point{Y: 0}
			},
		})

		shownY := track.DragOffset().Y
		if deck.ScrollPulling {
			shownY = deck.Offset.Y
		}

		var pullDist, pushDist float32
		if a.ActiveSpace == SpaceDashboard {
			pullDist = max(float32(0), shownY+h)
		} else {
			pushDist = max(float32(0), -shownY)
		}
		dragging := track.Dragging() || deck.ScrollPulling
		if dragging {
			a.CaptureDialOpen = false
			activeDist := pullDist
			if a.ActiveSpace == SpaceArchive {
				activeDist = pushDist
			}
			crossed := activeDist >= threshold
			if crossed && !deck.CrossedThreshold {
				mygo.Haptics.Light()
			}
			deck.CrossedThreshold = crossed
		} else {
			deck.CrossedThreshold = false
		}

		if !track.Dragging() && !deck.ScrollPulling {
			next := SpaceDashboard
			if deck.Offset.Y > -h*0.5 {
				next = SpaceArchive
			}
			if next != a.ActiveSpace {
				mygo.Haptics.Selection()
				a.CaptureDialOpen = false
				a.ActiveSpace = next
				deck.LastSpace = next
				c.AnimationFrame()
			}
		}

		showArchive := shownY > -h+0.5 || a.ActiveSpace == SpaceArchive
		showDashboard := shownY < -0.5 || a.ActiveSpace == SpaceDashboard

		track.Children(func() {
			// Upper floor (y = 0 .. h): Inspiration Archive Vault
			archiveSlot := ui.Box(c).Key("space_floor_archive").
				FillWidth().Height(h).Shrink(0).Clip().Background(pal.Canvas)
			if showArchive {
				archiveSlot.Children(func() {
					a.renderArchiveView(c, pushDist, threshold)
				})
			}

			// Lower floor (y = h .. 2h): Home Bento Dashboard
			dashSlot := ui.Box(c).Key("space_floor_dashboard").
				FillWidth().Height(h).Shrink(0).Clip().Background(pal.Canvas)
			if showDashboard {
				dashSlot.Children(func() {
					a.renderDashboardView(c, pullDist, threshold)
				})
			}
		})
	})
}

// handleSpaceBoundaryScroll smoothly hands off vertical touch pans or trackpad/wheel scrolls
// once an inner scroll container reaches its boundary (top of Dashboard -> pull Archive down;
// bottom of Archive -> push Archive up to Dashboard).
func (a *App) handleSpaceBoundaryScroll(ev ui.InputEvent, target Space, atBoundary bool) bool {
	if ev.Kind != ui.InputScroll {
		return false
	}
	deck := &a.SpaceDeck
	h := deck.ViewportH
	if h <= 0 {
		return false
	}

	if ev.Momentum {
		if !deck.ScrollPulling {
			return false
		}
		if ev.DY != 0 {
			v := -ev.DY * 60
			if math.Abs(float64(v)) > math.Abs(float64(deck.ScrollVelY)) {
				deck.ScrollVelY = v
			}
		}
		a.commitSpaceScrollPull(h)
		return true
	}

	dy := ev.DY
	if !ev.Precise {
		// Amplify discrete mouse wheel notches so desktop users can open/close effortlessly.
		dy *= 2.2
	}

	if deck.ScrollPulling {
		nextY := deck.Offset.Y - dy
		if deck.ScrollFrom == SpaceDashboard {
			if nextY <= -h {
				deck.Offset.Y = -h
				deck.ScrollPulling = false
				deck.ScrollVelY = 0
				return false
			}
			a.updateSpaceScrollPull(nextY, ev.Time, h)
			return true
		}
		if nextY >= 0 {
			deck.Offset.Y = 0
			deck.ScrollPulling = false
			deck.ScrollVelY = 0
			return false
		}
		a.updateSpaceScrollPull(nextY, ev.Time, h)
		return true
	}

	if !atBoundary {
		return false
	}

	if target == SpaceArchive && a.ActiveSpace == SpaceDashboard && dy < 0 {
		a.CaptureDialOpen = false
		deck.ScrollPulling = true
		deck.ScrollFrom = SpaceDashboard
		deck.ScrollVelY = 0
		deck.ScrollLastAt = ev.Time
		a.updateSpaceScrollPull(-h-dy, ev.Time, h)
		return true
	}
	if target == SpaceDashboard && a.ActiveSpace == SpaceArchive && dy > 0 {
		deck.ScrollPulling = true
		deck.ScrollFrom = SpaceArchive
		deck.ScrollVelY = 0
		deck.ScrollLastAt = ev.Time
		a.updateSpaceScrollPull(-dy, ev.Time, h)
		return true
	}
	return false
}

func (a *App) updateSpaceScrollPull(nextY float32, now time.Time, h float32) {
	deck := &a.SpaceDeck
	clamped := max(-h, min(float32(0), nextY))
	delta := clamped - deck.Offset.Y
	if !deck.ScrollLastAt.IsZero() && !now.IsZero() {
		dt := float32(now.Sub(deck.ScrollLastAt).Seconds())
		if dt > 0.002 && dt < 0.2 {
			instV := delta / dt
			deck.ScrollVelY = 0.5*deck.ScrollVelY + 0.5*instV
		}
	}
	deck.Offset.Y = clamped
	if !now.IsZero() {
		deck.ScrollLastAt = now
	}
}

func (a *App) commitSpaceScrollPull(h float32) {
	deck := &a.SpaceDeck
	deck.ScrollPulling = false
	threshold := spacePullThreshold(h)
	projectedY := max(-h, min(float32(0), ui.Project(deck.Offset.Y, deck.ScrollVelY)))

	next := deck.ScrollFrom
	if deck.ScrollFrom == SpaceDashboard {
		if projectedY+h >= threshold {
			next = SpaceArchive
		}
	} else {
		if -projectedY >= threshold {
			next = SpaceDashboard
		}
	}

	if next != a.ActiveSpace {
		mygo.Haptics.Selection()
		a.CaptureDialOpen = false
		a.ActiveSpace = next
		deck.LastSpace = next
	}
	deck.Offset.Y = spaceTargetY(next, h)
	deck.ScrollVelY = 0
}
