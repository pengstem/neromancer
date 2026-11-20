---
name: mygo-design
description: >-
  Guide and workflow for designing, visually comparing, and implementing
  cross-platform desktop and mobile UIs with egoist/mygo (pure-Go GPU Native UI
  and System WebView). Activate whenever designing a UI direction, page,
  component, shader, or interaction in MyGo: first benchmark ecosystem works
  (godraw, kero, gorex, pulse, lobe-mygo, MujicaUI, liubai), visually showcase
  2-3 distinct rendered designs side-by-side, ask the user which one they like
  best, and implement the chosen design.
---

# MyGo Design & Visual Exploration Skill (`egoist/mygo`)

This skill combines architectural patterns, UI craft, and a **visual-first multi-option selection workflow** distilled from the best open-source works in the **`github.com/egoist/mygo`** ecosystem (`egoist/mygo-showcase`, `echoVic/awesome-mygo-apps`, `mygodock.dev`, and official `mygo` packages).

---

## 1. Mandatory Workflow for Design Tasks: Benchmark -> Visualize -> Pick -> Implement

Whenever the user asks to design or redesign a direction, screen, component, or visual style (unless they explicitly dictate a single exact spec upfront), **always** follow these 4 steps instead of jumping straight into production code:

1. **Consult Ecosystem Benchmarks (先参阅不同流派)**:
   Read [references/showcase-apps.md](./references/showcase-apps.md) and [references/visual-comparison-protocol.md](./references/visual-comparison-protocol.md) to formulate **3 genuinely distinct design directions** rooted in proven `mygo` works (e.g., *Spatial & Liquid Glass* like `godraw`/`gorex` vs. *Structured High-Clarity* like `lobe-mygo`/`kero` vs. *Calm Editorial Paper* like `liubai`/`MujicaUI`).
2. **Showcase Visually, Not in Walls of Text (把各个设计直观展现出来)**:
   Follow [references/visual-comparison-protocol.md](./references/visual-comparison-protocol.md) to render all candidate designs **visually** before touching production code:
   - **True Pixel Previews (Preferred)**: Render each `mygo/ui` candidate offscreen at `2x` Retina via `ui.NewTester` / `ui.Render` into PNGs inside the conversation's Artifact Directory, and display them in a Markdown Artifact ````carousel` (or side-by-side). Clean up any temporary preview test file immediately after exporting the PNGs.
   - **Interactive Switcher (Alternative)**: For multi-state/interactive concept comparisons, build a compact `generative_ui` HTML tab-switcher artifact (`方案 A / B / C`).
3. **Single Choice Gate — Ask Only Which One They Prefer (只问最喜欢哪一个)**:
   Call `ask_question` with **one concise question** listing the visual candidates (putting `(Recommended)` on the strongest fit; the UI automatically supplies an "Other" option if the user wants to mix elements like "A's layout + C's palette"). **Stop and wait** for the user's pick.
4. **Implement & Verify the Chosen Design (选定即落地)**:
   Once the user chooses, implement that exact direction in production code following the tenets below, add `ui.NewTester` coverage, and run [scripts/verify-mygo-ui.sh](./scripts/verify-mygo-ui.sh).

---

## 2. Reference Index (Progressive Disclosure)

Load only the reference files needed for the current stage:

- **Visual multi-option preview & `ask_question` templates**:
  [references/visual-comparison-protocol.md](./references/visual-comparison-protocol.md) — `ui.NewTester` / `ui.Render` `2x` PNG export snippet, Artifact ````carousel` template, `generative_ui` switcher pattern, and selection gate rules.
- **Benchmark works & product/UX archetypes**:
  [references/showcase-apps.md](./references/showcase-apps.md) — decision matrix + deep dives into `godraw`, `kero`, `gorex`, `pulse`, `lorca`, `lobe-mygo`, `MujicaUI`, `liubai`, `mygo-music`, `pinky`, and `egoist/mygo/examples/*`.
- **Component signatures, headless `*Base` primitives, tokens & mobile/desktop layouts**:
  [references/ui-architecture-and-libs.md](./references/ui-architecture-and-libs.md) — `(c, content..., Props)` zero-value convention, `ButtonBase`/`DialogBase`/`PopoverBase` mapping, `ui.Oklch` layered surfaces, and `ui/mobile` + desktop companion patterns.
- **Kage GPU shaders, Liquid Glass, `Painter` & deterministic UI testing**:
  [references/gpu-shaders-and-testing.md](./references/gpu-shaders-and-testing.md) — `plugins/glass` (`glass.Glass{}`), premultiplied Kage shaders, `pixelScale()` anti-aliasing, `DrawInstances`, `ui/golden`, `plugins/automation`, and `go tool mygo vet`.

---

## 3. Core MyGo Implementation Tenets

1. **Respect Single-Frame Handle Lifetimes**:
   - `ui.Element` and `*ui.Context` scope expire every build pass. Never store them in struct fields, long-lived closures, or goroutines.
   - Persist pure data on your App struct, `ui.Handle` (`ui.Ref`) for focus/bounds identity, `ui.ListState` / `ui.ScrollState` for scrolling, and `.Key("prefix_" + item.ID)` on every dynamic loop item.
2. **Headless `*Base` Primitives + `(c, content..., Props)` Signature**:
   - Build custom visuals atop `ButtonBase`, `SwitchBase`, `SelectBase`, `TabsBase`, `DialogBase`, `PopoverBase`, `ToastViewportBase`, and `TextInputBase` so keyboard focus, IME, and screen-reader semantics come free.
   - Keep `*ui.Context` first, required content positional, and optional settings in a trailing `Props` struct whose zero value `Props{}` is the clean default.
3. **Semantic Tokens & Intentional Craft (Anti-Slop)**:
   - Centralize colors (`ui.Oklch` / `ui.Hex`), fonts (`ui.RegisterFont`), spacing, and radii in semantic tokens; use pre-parsed vector SVGs (`ui.MustParseSVG`) instead of emojis, `1px` hairline alpha borders, and `Element.Spring` physical feedback.
4. **Event-Driven Idle Discipline**:
   - Call `Painter.AnimationFrame()` only while motion is active; run network/AI/file I/O off the main thread and dispatch state updates via `win.Update(func() { ... })` or `mygo.RunOnMain`.
5. **Mandatory Verification**:
   - Run `go doc github.com/egoist/mygo/ui.<Symbol>` when unsure of an API, and always run [scripts/verify-mygo-ui.sh](./scripts/verify-mygo-ui.sh) (`go vet`, `go tool mygo vet`, `go test -race`) after UI edits.
