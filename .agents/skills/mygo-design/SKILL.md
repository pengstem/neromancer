---
name: mygo-design
description: >-
  Guide and reference for designing, architecting, and polishing cross-platform
  desktop and mobile applications with egoist/mygo (pure-Go GPU Native UI and
  System WebView). Use this skill whenever designing UI/UX, building custom
  components, writing Kage GPU shaders or Liquid Glass materials, structuring
  window/tray/mobile interactions, or seeking inspiration and proven patterns
  from top MyGo ecosystem works (godraw, kero, gorex, pulse, lobe-mygo,
  MujicaUI, liubai, and official examples).
---

# MyGo Design & Architecture Skill (`egoist/mygo`)

This skill distills architectural patterns, UI component conventions, visual craft, and verification workflows from the best open-source works in the **`github.com/egoist/mygo`** ecosystem (`egoist/mygo-showcase`, `echoVic/awesome-mygo-apps`, `mygodock.dev`, and official `mygo` packages).

## 1. Choose the Right Reference First (Progressive Disclosure)

Read only the reference file matching your current task before writing code:

- **Looking for product/UX inspiration or architectural precedents?**
  Read [references/showcase-apps.md](./references/showcase-apps.md) — covers `godraw` (infinite canvas & immutable undo), `kero` & `gorex` (spatial card terminals & daemon decoupling), `pulse` (tray + flyout telemetry), `lorca` (desktop + mobile shared views), `mygo-music` / `mixmusic` (plugin stores & media), and `liubai` (minimalist local-first editor).
- **Designing custom widgets, design tokens, themes, or mobile/desktop layouts?**
  Read [references/ui-architecture-and-libs.md](./references/ui-architecture-and-libs.md) — covers headless `*Base` primitives (`ButtonBase`, `DialogBase`, `PopoverBase`, etc.), `lobehub/lobe-mygo`'s `(c, required..., Props)` signature & semantic token model, `MujicaUI`'s editorial palettes & density scales, and `mygo/ui/mobile` adaptive patterns.
- **Building Kage GPU shaders, Liquid Glass, custom `Painter` visuals, or headless UI tests?**
  Read [references/gpu-shaders-and-testing.md](./references/gpu-shaders-and-testing.md) — covers `mygo/ui/shader` (`Fragment`, `DrawInstances`, multi-pass `ui.Canvas`), `mygo/plugins/glass` (`glass.Glass{}`, `ScrollEdge`), `ui.Painter`, `ui.NewTester`, `ui/golden`, `plugins/automation`, and `go tool mygo vet`.

## 2. Core MyGo Design Tenets

1. **Pick the Rendering Mode Deliberately**:
   - **Native GPU UI (`github.com/egoist/mygo/ui`)**: Default choice for fast startup, ~44 MB idle RAM, ~0% idle CPU, tactile spring physics, Kage shaders, Liquid Glass, and unified Desktop + Android/iOS/Wasm targets.
   - **System WebView + Typed IPC (`mygo.Bind` + `mygo generate`)**: Choose only when embedding rich third-party web ecosystems (e.g., complex web editors or JS/TS plugin sandboxes as in `egoist/mygo-music`). Windows of both modes can coexist in one app.
2. **Respect Single-Frame Handle Lifetimes**:
   - `ui.Element` (16-byte pooled handle) and `*ui.Context` scope are strictly single-pass/single-frame. Never store them in struct fields, closures that outlive the frame, or goroutines.
   - Persist **pure data state** on your App/View struct, `ui.Handle` (`ui.Ref`) for focus/bounds identity, `ui.ListState` / `ui.ScrollState` for virtualized scrolling, and `.Key("prefix_" + item.ID)` on dynamic sibling loops.
3. **Build Custom Visuals on Headless `*Base` Primitives**:
   - Never reinvent pointer capture, keyboard navigation, IME composition, focus rings, or screen-reader semantics with raw `ui.Box(c)` when a `*Base` primitive exists (`ButtonBase`, `CheckboxBase`, `SwitchBase`, `SelectBase`, `TabsBase`, `DialogBase`, `PopoverBase`, `ToastViewportBase`, `TextInputBase`).
4. **Follow the `(c, content..., Props)` Component Signature (`lobe-mygo` Pattern)**:
   - Put `*ui.Context` first, required content positionally second, and optional configuration in a trailing `Props` struct whose zero value `Props{}` produces the clean default variant.
5. **Semantic Design Tokens + Intentional Craft (Anti-Slop)**:
   - Centralize colors (`ui.Oklch` / `ui.Hex`), typography (`ui.RegisterFont`), spacing, and radii in a semantic token struct (`Surface`, `SurfaceElevated`, `TextPrimary`, `TextMuted`, `BorderHairline`, `Accent`) rather than scattering raw hex codes.
   - Replace generic emoji icons with crisp vector paths (`ui.MustParseSVG` + `ui.Icon` or `Painter` vector strokes), hairline borders (`1px` alpha-blended), and subtle physical motion (`Element.Spring`).
6. **Event-Driven Idle Discipline**:
   - Never run busy `time.Sleep` redraw loops. Trigger `Painter.AnimationFrame()` only while an animation or shader is actively moving; move I/O and LLM calls off the main thread and marshal state updates back via `win.Update(func() { ... })` or `mygo.RunOnMain`.

## 3. Design-to-Implementation Workflow

1. **Map Archetype & Layout**: Identify the closest ecosystem archetype in [references/showcase-apps.md](./references/showcase-apps.md) (e.g., spatial cards, sidebar + stage, mobile bottom-bar + sheet, or tray + frameless flyout).
2. **Inspect Exact Local APIs**: Run `go doc github.com/egoist/mygo/ui.<Symbol>` (or subpackages `ui/mobile`, `ui/shader`, `plugins/glass`) to confirm method names in the workspace's pinned `mygo` version.
3. **Compose Small View Files**: Split each distinct screen, sheet, or custom widget into its own focused file with semantic tokens and explicit `.Key(...)` on dynamic items.
4. **Verify Lifetime Safety & Visual Quality**:
   Run the verification script [scripts/verify-mygo-ui.sh](./scripts/verify-mygo-ui.sh) (executes `go vet`, `go tool mygo vet`, and `go test -race`), and cover new views with deterministic `ui.NewTester` + `ui/golden` assertions.
