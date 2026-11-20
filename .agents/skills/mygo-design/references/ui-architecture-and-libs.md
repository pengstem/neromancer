# MyGo Native UI Architecture, Component Systems & Adaptive Layouts

Reference guide for structuring `github.com/egoist/mygo/ui` and `github.com/egoist/mygo/ui/mobile` codebases cleanly, borrowing proven patterns from `lobehub/lobe-mygo`, `MujicaUI`, `godraw`, and `mygo` core.

---

## 1. The Mental Model of `package ui`

`mygo/ui` is an **immediate-syntax, retained-state, GPU-rendered** toolkit (~0.8 µs per element build; ~0% CPU when idle):

1. **Views are pure frame functions of app state**:
   `func (a *App) RootView(c *ui.Context)` runs on the OS main thread whenever input arrives, `win.Update(...)` / `Invalidate` fires, or an animation ticks.
2. **Handles expire every build pass**:
   - `ui.Element` is a 16-byte checked handle into a pooled node arena. It becomes invalid before the next pass (one frame can run multiple layout/build passes).
   - **Never** store `ui.Element`, `*ui.Context`, or `*Base` parts structs in app fields, slices, maps, or goroutines.
   - For persistent control identity (focusing an input programmatically, reading committed bounds, routing commands), store a `ui.Handle` (`var searchRef ui.Handle`) on your state struct.
   - For element-scoped ephemeral UI state (e.g., hover timers or local drag offsets inside a reusable component), use `ui.Local(elem, key, func() T { ... })`.
3. **Inline Input Semantics (Dear ImGui / egui style)**:
   - Controls process pending input **during construction** and update bound pointers (`*string`, `*bool`, `*int`, `*float64`) before returning.
   - Methods like `.Clicked()`, `.Changed()`, `.Submitted()`, `.Hovered()`, `.Active()` answer immediately in `if` statements, while `.OnClick(fn)` / `.OnChange(fn)` run cleanly before rebuild.
4. **Sibling Position vs `.Key(...)`**:
   - Between frames, MyGo matches focus, scroll offset, active text composition, and spring state by an element's sibling index—**or its explicit `.Key(...)`**.
   - **Rule**: Every loop over dynamic/reorderable/filterable data MUST attach `.Key("item_" + item.ID)` to the root element of each iteration.

---

## 2. Component Signature Convention (`lobehub/lobe-mygo` Standard)

Scale custom components cleanly without builder-method sprawl by adopting the 3-part signature:

```go
// 1. Props struct where the zero value Props{} is always valid and sensible.
type CardVariant uint8

const (
	CardElevated CardVariant = iota // zero value = default!
	CardOutlined
	CardSubtle
)

type IdeaCardProps struct {
	Key      string
	Variant  CardVariant
	Selected bool
	Compact  bool
}

// 2. Signature: (*ui.Context, required positional args..., trailing Props) ui.Element
func IdeaCard(c *ui.Context, title string, summary string, p IdeaCardProps, extra func()) ui.Element {
	tok := TokensOf(c)
	pad := tok.SpaceMD
	if p.Compact {
		pad = tok.SpaceSM
	}

	card := ui.ButtonBase(c)
	if p.Key != "" {
		card = card.Key(p.Key)
	}

	bg := tok.BgContainer
	border := tok.BorderSubtle
	if p.Selected {
		bg = tok.BgSelected
		border = tok.AccentBorder
	} else if card.Hovered() {
		bg = tok.BgHover
	}

	// Smooth physical press feedback via Element.Spring
	scale := card.Spring("press", ternaryFloat(card.Active(), 0.985, 1.0))

	return card.
		Column().
		WidthPercent(100).
		Padding(pad).
		Gap(tok.SpaceXS).
		Radius(tok.RadiusLG).
		Background(bg).
		Border(1, border).
		Scale(scale, scale).
		Children(func() {
			ui.Text(c, title).FontSize(tok.FontTitle).Bold().TextColor(tok.TextPrimary)
			if summary != "" {
				ui.Text(c, summary).FontSize(tok.FontBody).TextColor(tok.TextSecondary)
			}
			if extra != nil {
				extra()
			}
		})
}
```

---

## 3. Headless `*Base` Primitives Over Raw `ui.Box`

MyGo separates **behavior & accessibility** (`*Base`) from **visual skin**. Whenever designing a custom-styled interactive control, start from its `*Base` constructor so keyboard navigation (`Tab`, arrows, `Enter`/`Space`, `Esc`), pointer capture, focus rings, and screen readers work automatically:

| Custom Component Goal | Use This Headless Primitive | What It Handles Automatically |
| :--- | :--- | :--- |
| Custom cards, icon buttons, pills, FABs | `ui.ButtonBase(c)` | Click/tap activation, Space/Enter keys, `:hover`/`:active` state, RoleButton accessibility, focus ring |
| Custom switches / filter chips | `ui.SwitchBase(c, &on)` / `ui.ToggleBase(c, &on)` | Boolean toggle state, ARIA checked/pressed state, keyboard toggle |
| Custom checkboxes | `ui.CheckboxBase(c, &checked)` | Checked state + label association + keyboard activation |
| Custom segmented controls / tab bars | `ui.SegmentedBase(c, &idx, n)` / `ui.TabsBase(c, &idx, n)` | Arrow-key roving tabindex, ARIA tablist/tabpanels semantics |
| Custom dropdowns & command pickers | `ui.SelectBase(c, &val)` / `ui.ComboboxBase(c, &text)` | Popup positioning, typeahead/filtering, arrow selection, Esc close |
| Custom modals & drawers | `ui.DialogBase(c, &open, func(backdrop, panel ui.Element))` | Focus trapping, Esc dismissal, backdrop click, ARIA modal layer |
| Custom popovers, menus, tooltips | `ui.PopoverBase`, `ui.MenuBase`, `ui.ContextMenuBase`, `ui.TooltipBase` | Viewport collision flipping, anchor tracking, menu keyboard nav |
| Custom search / chat composer inputs | `ui.TextInputBase(c, &s)` / `ui.TextAreaBase(c, &s)` | Full CJK IME composition, selection, clipboard, undo coalescing |
| Non-blocking notifications | `ui.ToastViewportBase` + `ui.ToastBase` | Stacking, timers, keyboard accessibility |

---

## 4. Semantic Design Tokens & Visual Polish

Borrowing from `lobehub/lobe-mygo` and `MujicaUI`:

1. **Perceptually Uniform Palettes (`ui.Oklch` & Layered Surfaces)**:
   - Prefer `ui.Oklch(l, c, h)` or curated warm/cool neutrals over pure `#000000` / `#FFFFFF`.
   - Define at least 3 surface elevation steps (`BgCanvas` -> `BgContainer` -> `BgElevated`) + hairline alpha borders (`ui.RGBA(r, g, b, 0.08)` in light mode, `ui.RGBA(255, 255, 255, 0.09)` in dark mode) so depth reads cleanly without muddy drop shadows.
2. **Tabular Metrics & Typography**:
   - Register brand/editorial fonts once at startup with `ui.RegisterFont(ttfBytes, "FamilyName")`.
   - Keep font scales tight and intentional (e.g., `11` caption / metadata, `13–14` body, `16–18` card/section title, `22–28` hero heading).
   - Prevent layout shift on live counters, timers, and audio durations by using fixed-width containers or monospace/tabular numerals.
3. **Vector Iconography (`ui.MustParseSVG`)**:
   - Parse SVGs once at package init (`var iconMic = ui.MustParseSVG(...)`) and render via `ui.Icon(c, iconMic).Size(18, 18).TextColor(...)`—never re-parse SVG strings inside a frame function, and never substitute emojis for UI icons.
4. **Virtualize Long Collections**:
   - Use `ui.List(c, &a.listState, len(items), func(i int) { ... })` or `ui.GridView(c, &a.gridState, ...)` when item counts can grow beyond ~50 rows.

---

## 5. Adaptive Desktop + Mobile Architecture (`lorca` & Neromancer Pattern)

When a single codebase runs on phones (`android`, `ios`) and desktop (`darwin`, `windows`, `linux`):

1. **Mobile Primitives (`github.com/egoist/mygo/ui/mobile`)**:
   - Use `mobile.TopBar`, `mobile.BottomBar`, `mobile.Sheet` (bottom sheet with drag-to-dismiss), `mobile.ActionSheet`, `mobile.SwipeRow` (swipe actions on list items), `mobile.PageView` + `mobile.PageDots`, `mobile.Ink` (touch feedback), and `mobile.Zoomable` (pinch-to-zoom).
   - `ui/mobile` is built purely on `ui`'s public API—it works smoothly with mouse input during desktop preview (`412x892` portrait window in `go tool mygo dev`) while automatically respecting phone safe areas on Android/iOS.
2. **Desktop Companion Superpowers**:
   - Gate desktop-only capabilities with `runtime.GOOS != "android" && runtime.GOOS != "ios" && runtime.GOOS != "js"`:
     - `mygo.NewTray` + `mygo.NewFlyout` for instant menu-bar access.
     - Frameless transparent `AlwaysOnTop` companion windows (`Frameless: true, Transparent: true, Panel: true, SkipTaskbar: true, BackgroundColor: "#00000000"`).
     - `mygo.GlobalShortcut` (e.g., `CmdOrCtrl+Shift+Space` / `CmdOrCtrl+Shift+I`) and custom deep-link URL schemes (`mygo.App.OnOpenURL`).
3. **Async Concurrency Boundary**:
   - Event callbacks and `view(c)` run on the locked main OS thread—keep them non-blocking (< 2 ms).
   - Spawn network/AI/disk work in a goroutine, and mutate app UI state **only** inside `win.Update(func() { ... })` or `mygo.RunOnMain(func() { ... })`.
