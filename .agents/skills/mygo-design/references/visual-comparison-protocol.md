# Multi-Option Visual Design Exploration Protocol

Whenever the user asks to design a page, component, layout, or visual direction (and has not already locked in a single exact specification), **never jump straight to rewriting production UI files or dumping abstract prose**. Execute this 4-stage visual selection loop instead.

---

## Stage 1: Synthesize 3 Distinct Benchmark-Backed Directions

Consult [showcase-apps.md](./showcase-apps.md) (and `search_web` if exploring a brand-new domain) to craft **3 genuinely distinct candidates**—differing in spatial structure, interaction model, and material language, not just accent colors:

| Slot | Archetype Lineage | Signature Characteristics | Best When User Values... |
| :--- | :--- | :--- | :--- |
| **Option A** | **Spatial & Tactile Glass** (`godraw` / `gorex` / `examples/liquid-glass`) | Full-bleed stage or card-framed splits, floating `glass.Glass{}` dock/pills, Kage shader micro-feedback | Immersive, modern, tactile, spatial focus |
| **Option B** | **Structured High-Clarity Tokens** (`lobehub/lobe-mygo` / `kero` / `pulse`) | Crisp `1px` hairline surfaces, segmented/pill controls, tabular metrics, compact scannable hierarchy | Speed, high information density, zero ambiguity |
| **Option C** | **Calm Editorial / Paper** (`echoVic/liubai` / `MujicaUI` / `examples/notes`) | Warm ivory or deep charcoal surfaces, generous breathing room, typography-led hierarchy, minimal chrome | Reading/writing comfort, warmth, timeless personality |

*(Adapt the 3 slots to the specific feature being designed—for example, for a quick-capture flow: Bottom Sheet vs. Floating Command Pill vs. Full-Screen Card).*

---

## Stage 2: Render Visual Previews (Show, Don't Just Tell)

Present all 3 candidates **visually** before touching production files, using whichever of these two methods best fits the task:

### Method 1 (Preferred for Real `mygo/ui` Pixel Fidelity): Offscreen `ui.Render` / `ui.Tester` PNGs + Artifact Carousel
MyGo renders any `func(c *ui.Context)` directly to a Retina `2x` `*image.RGBA` in memory without opening an OS window.

1. Create a temporary test file (e.g., `internal/ui/design_preview_tmp_test.go`, **removed right after exporting PNGs**) that renders each candidate view directly into the conversation's **Artifact Directory** (`<appDataDir>/brain/<conversation-id>/`):

```go
package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func renderCandidatePNG(t *testing.T, outPath string, w, h int, view func(c *ui.Context)) {
	t.Helper()
	tt := ui.NewTester(view, w, h)
	tt.SetScale(2.0) // crisp 2x Retina preview
	tt.Frame()
	for i := 0; i < 6 && tt.FramePending(); i++ {
		tt.Advance(80 * time.Millisecond)
	}
	tt.Frame()

	f, err := os.Create(outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestExportDesignCandidatesTmp(t *testing.T) {
	outDir := os.Getenv("MYGO_PREVIEW_OUT_DIR")
	if outDir == "" {
		t.Skip("MYGO_PREVIEW_OUT_DIR not set")
	}
	renderCandidatePNG(t, filepath.Join(outDir, "option_a_spatial.png"), 412, 892, previewOptionA)
	renderCandidatePNG(t, filepath.Join(outDir, "option_b_structured.png"), 412, 892, previewOptionB)
	renderCandidatePNG(t, filepath.Join(outDir, "option_c_editorial.png"), 412, 892, previewOptionC)
}
```

2. Run `MYGO_PREVIEW_OUT_DIR="<artifact_dir>" go test -run TestExportDesignCandidatesTmp ./internal/ui` and delete `design_preview_tmp_test.go` so the working tree stays clean while waiting for the user's choice.
3. Create a visual comparison Markdown Artifact (`design_options.md` in `<artifact_dir>`, `UserFacing: true`, `RequestFeedback: false`) embedding the rendered PNGs inside a ````carousel` block (using absolute paths already inside `<artifact_dir>`):

`````markdown
````carousel
### 方案 A：空间悬浮与液态玻璃（借鉴 `godraw` / `liquid-glass`）
- **核心特点**：全屏沉浸画布 + 底部悬浮毛玻璃胶囊栏，视觉呼吸感最强。

![方案 A 预览](/absolute/artifact/dir/option_a_spatial.png)
<!-- slide -->
### 方案 B：高密度结构化卡片（借鉴 `lobe-mygo` / `kero`）
- **核心特点**：1px 细线分层卡片 + 紧凑分段导航，一眼扫完关键信息。

![方案 B 预览](/absolute/artifact/dir/option_b_structured.png)
<!-- slide -->
### 方案 C：留白纸感排版（借鉴 `liubai` / `MujicaUI`）
- **核心特点**：暖白纸张底色 + 字体层级主导，像翻阅随身手账一样安静。

![方案 C 预览](/absolute/artifact/dir/option_c_editorial.png)
````
`````

### Method 2 (Fast Interactive Concept Switcher): `generative_ui` HTML Widget
When exploring interactive layout mechanics, motion states, or multi-step flows before writing Go code, read the `generative_ui` skill and write a self-contained HTML switcher artifact (`design_switcher.html` in `<artifact_dir>`) with top pill tabs (`方案 A` / `方案 B` / `方案 C`) so the user can click between live interactive mockups directly in chat (`<agent-embed>` if `<= 450px` tall) or in the side pane (if full-height).

---

## Stage 3: Ask ONLY "Which One Do You Prefer?" (`ask_question`)

Once the visual showcase is ready:
1. Keep your chat message brief: point to the visual showcase and state each option's 1-line essence (do **not** repeat a wall of text).
2. Immediately call the `ask_question` tool with **a single clear selection question**—do not bombard the user with 5 micro-questions:
   - Put your best-fit recommendation first, prefixed with `(Recommended)`.
   - List each option concisely by name + 1 short phrase (e.g., `"(Recommended) 方案 A：空间悬浮毛玻璃（godraw / liquid-glass 风格）"`, `"方案 B：高密度结构化卡片（lobe-mygo / kero 风格）"`, `"方案 C：留白纸感手账（liubai / MujicaUI 风格）"`).
   - Remember `ask_question` automatically provides an "Other" free-text input in the UI, so the user can effortlessly reply "选 A，但把配色换成 C 的暖白色" if they want a hybrid.
3. **Stop and wait for the user's answer** before modifying production files.

---

## Stage 4: Adopt & Land in Production

As soon as the user picks their favorite (or specifies a hybrid):
1. Implement the chosen design cleanly in `internal/ui/` using headless `*Base` primitives, `(c, content..., Props)` conventions, semantic tokens, and `.Key(...)` on dynamic items.
2. Add or update headless `ui.NewTester` coverage for the new view.
3. Run [../scripts/verify-mygo-ui.sh](../scripts/verify-mygo-ui.sh) (`go vet ./...`, `go tool mygo vet`, `go test -race ./...`) and build to verify zero warnings or regressions.
