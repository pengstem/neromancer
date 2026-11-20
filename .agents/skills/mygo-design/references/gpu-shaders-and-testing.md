# GPU Shaders, Liquid Glass, Custom Painting & Deterministic UI Testing

Technical reference for `mygo/ui/shader`, `mygo/plugins/glass`, `ui.Painter`, `ui.NewTester`, `ui/golden`, `plugins/automation`, and `go tool mygo vet`.

---

## 1. Liquid Glass & Backdrop Materials (`github.com/egoist/mygo/plugins/glass`)

MyGo includes cross-platform GPU/CPU shaders reproducing macOS 26+ Liquid Glass (`NSGlassEffectView`) identically across macOS, Windows, Linux, Android, iOS, and WebGL2:

```go
import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// 1. Floating dock, pill bar, or overlay card with lens rim refraction & specular edge:
ui.Row(c).
	Padding(8, 16).
	Gap(12).
	Radius(24).
	Material(glass.Glass{}).
	Children(func() {
		// dock items...
	})

// 2. Scroll edge frosting under sticky headers / bottom bars:
// Use glass.ScrollEdge{} or glass.Blur{} so scrolling cards softly blur underneath bars.
```

**Design Guidelines for Glass**:
- Reserve `glass.Glass{}` for **floating chrome layers** (bottom docks, floating action pills, transient sheets/HUDs) sitting above rich content (orbs, imagery, cards). Never stack glass-on-glass-on-glass, which muddies legibility.
- Ensure text/icons atop glass maintain strong contrast across light and dark backgrounds.

---

## 2. Custom Kage Fragment Shaders (`github.com/egoist/mygo/ui/shader`)

MyGo compiles Go-syntax Kage shaders (`package main`, `func Fragment(dst vec4, pos vec2) vec4`) directly to Metal (MSL), Direct3D 11 (HLSL), OpenGL (GLSL), WebGL 2, and a fallback CPU interpreter.

### Key Rules & Performance Patterns:
1. **Compile Once at Package Init + Pre-Resolve Uniform Handles**:
   - Always call `shader.MustCompile(...)` in a package-level `var` and cache `s.Uniform("VarName")` handles at init time—never compile shaders or look up uniform strings per frame!
2. **Premultiplied Alpha Output**:
   - `Fragment(dst vec4, pos vec2) vec4` MUST return **premultiplied RGBA** (`vec4(rgb * alpha, alpha)`). Returning straight alpha with `alpha < 1` causes glowing fringes/halos at anti-aliased edges.
3. **DPI-Aware Anti-Aliasing (`pixelScale()`)**:
   - `pos` and `rectSize()` are in logical **DIPs**, while `dst` is in physical pixels.
   - Compute crisp 1-pixel SDF edges across Retina/HiDPI displays using `aa := 1.0 / max(pixelScale(), 1.0)` with `smoothstep`.
4. **Batched Instancing (`DrawInstances`) for Spatial Canvases**:
   - When rendering many orbs, nodes, particles, or strokes (as in `godraw` or mind-space views), use `Painter.ShaderInstances` / `shader.Instances` (`instanceIndex()`, `instanceData(0..15)`) to draw hundreds of parameterized quads in a single GPU draw call.
5. **Idle Frame Pacing**:
   - Call `p.AnimationFrame()` (or `p.AnimationFrameRate(30)` for subtle ambient drift) inside `Element.Draw(func(p *ui.Painter, r ui.Rect))` **only** when visual state is actively animating, and honour `p.Preferences().ReducedMotion` if applicable.

```go
var pulseShader = shader.MustCompile(`package main

var Accent vec3
var Phase float

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	uv := (pos - 0.5*size) / (0.5 * min(size.x, size.y))
	d := length(uv)
	aa := 2.0 / max(min(size.x, size.y)*pixelScale(), 1.0)

	ring := smoothstep(0.88+aa, 0.88-aa, d) * smoothstep(0.72-aa, 0.72+aa, d)
	wave := 0.65 + 0.35*sin(Phase*3.0 - d*6.0)
	a := clamp(ring*wave, 0.0, 1.0)
	return vec4(Accent*a, a) // premultiplied!
}
`)

var (
	pulseAccent = pulseShader.Uniform("Accent")
	pulsePhase  = pulseShader.Uniform("Phase")
)
```

---

## 3. Custom Vector Drawing & Offscreen Export (`ui.Painter` & `ui.Render`)

1. **`Element.Draw` vs `Element.DrawOver`**:
   - `.Draw(func(p *ui.Painter, r ui.Rect))` paints behind the element's children (custom backgrounds, grid lines, sparklines, shaders).
   - `.DrawOver(func(p *ui.Painter, r ui.Rect))` paints in front of children (selection overlays, connector arrows, badges, custom focus rings).
   - Note: `r` coordinates passed to `Painter` are in **window DIPs** (`r.X`, `r.Y`, `r.W`, `r.H`). Always offset relative to `r.X` and `r.Y`, never `(0, 0)`!
2. **Offscreen PNG Poster / Card Rasterization (`ui.Render`)**:
   - `img := ui.Render(shareCardView, width, height, scale)` renders any `mygo/ui` view function directly to an `*image.RGBA` in memory without opening an OS window—ideal for high-DPI share cards, export thumbnails, and visual audits.

---

## 4. Three-Tier Verification & Visual Regression Pipeline

### Tier 1: Static Lifetime Verification (`go tool mygo vet`)
Runs `github.com/egoist/mygo/internal/uivet` using full Go `types.Info` to catch illegal storage or cross-frame capture of `ui.Element`, `*ui.Context`, and `*Base` parts structs. **Always run after every UI edit.**

### Tier 2: Deterministic Headless UI Tests (`ui.NewTester` + `ui/golden`)
`ui.Tester` runs views in memory with a virtual clock (`tt.Advance(d)`), simulated mouse/touch/pinch/keyboard/IME input, dark-mode switching (`tt.SetDark(true)`), safe-area insets (`tt.SetInsets(...)`), accessibility tree queries (`tt.Access`, `tt.AccessByRole`), and pixel output (`tt.Image()`):

```go
func TestCaptureSheetFlow(t *testing.T) {
	app := newTestApp(t)
	tt := ui.NewTester(app.RootView, 412, 892)

	// Settle initial entrance animations deterministically
	for i := 0; i < 6 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
	}

	if err := tt.Click("记灵感"); err != nil {
		t.Fatalf("open capture sheet: %v", err)
	}
	tt.Type("用 MyGo Headless Base 重构卡片组件")
	if !tt.HasText("用 MyGo Headless Base 重构卡片组件") {
		t.Fatal("typed text not visible in capture input")
	}

	// Optional pixel-level golden regression (update with MYGO_UPDATE_GOLDEN=1):
	// golden.Match(t, tt, "testdata/capture_sheet.png")
}
```

### Tier 3: Live Window E2E Socket (`mygo/plugins/automation`)
For multi-window desktop interactions, add an opt-in `//go:build e2e` file importing `_ "github.com/egoist/mygo/plugins/automation"` and launch with `MYGO_AUTOMATION=/tmp/app.sock` to inspect elements, trigger menus, send input, and capture live window PNGs externally (automatically stripped in release `mygo build`).
