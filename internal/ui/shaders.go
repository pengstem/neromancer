package ui

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/shader"
	"github.com/pengstem/neromancer/internal/domain"
)

var orbShader = shader.MustCompile(`package main

var Core vec3
var Mid vec3
var Rim vec3
var Phase float
var Energy float
var RadiusRatio float
var Selected float

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	s := min(size.x, size.y)
	rr := max(RadiusRatio, 0.5)
	inner := s * 0.5 * rr
	uv := (pos - 0.5*size) / max(inner, 1.0)
	dist := length(uv)
	maxDist := 1.0 / rr

	haloFade := smoothstep(maxDist, 1.01, dist)
	halo := exp(-max(dist-0.95, 0.0)*4.8) * (0.24 + 0.18*Energy + 0.12*Selected) * haloFade
	haloCol := mix(Mid, Rim, 0.45) * halo

	edge := 1.35 / max(inner*pixelScale(), 1.0)
	if dist > 1.0+edge {
		return vec4(clamp(haloCol, 0.0, 1.0), clamp(halo, 0.0, 1.0))
	}

	z := sqrt(max(1.0-min(dist*dist, 1.0), 0.0))
	n := vec3(uv, z)
	t := Phase

	// Internal swirling glass caustics driven by Core/Mid/Rim + Phase + Energy.
	w := uv * (1.12 + 0.18*z)
	w += (0.20 + 0.22*Energy) * vec2(sin(w.y*2.8+t*1.05), cos(w.x*2.4-t*0.92))
	w += 0.10 * vec2(sin(w.y*5.1-t*1.6), cos(w.x*4.6+t*1.3))

	c0 := 0.44 * vec2(sin(t*0.65), cos(t*0.52))
	c1 := 0.48 * vec2(sin(t*0.78+2.1), cos(t*0.61+1.3))
	c2 := 0.42 * vec2(sin(t*0.53+4.2), cos(t*0.84+2.9))
	k0 := exp(-dot(w-c0, w-c0) * 2.8)
	k1 := exp(-dot(w-c1, w-c1) * 3.1)
	k2 := exp(-dot(w-c2, w-c2) * 3.4)

	col := mix(Rim*0.56+Mid*0.28, Mid*0.86+Core*0.18, 0.36+0.64*z)
	col = mix(col, Mid, clamp(k1*0.72, 0.0, 0.85))
	col += Core * k0 * (0.28 + 0.16*Energy)
	col += Rim * k2 * 0.28

	// Transmitted internal caustic glow toward lower-right.
	refractPos := uv - vec2(0.24, 0.28)
	col += Core * exp(-dot(refractPos, refractPos)*4.6) * (0.16 + 0.10*Energy)

	// Glass depth, Fresnel rim glow, and top-left specular catchlight.
	col *= 0.84 + 0.22*z
	fresnel := pow(1.0-z, 2.4)
	col += mix(Mid, Core, 0.45) * fresnel * (0.68 + 0.24*Selected)

	// Soft tone compression before sharp specular highlights.
	col = col / (vec3(1.0) + col*0.28)

	lightDir := normalize(vec3(-0.42, -0.52, 0.74))
	specSharp := pow(max(dot(n, lightDir), 0.0), 54.0)
	specSoft := pow(max(dot(n, normalize(vec3(-0.34, -0.44, 0.83))), 0.0), 10.0)
	col += vec3(1.0) * (specSharp*0.85 + specSoft*0.16)
	col += vec3(1.0) * 0.08 * smoothstep(0.25, 0.95, -uv.y*z)

	sphereA := smoothstep(1.0+edge, 1.0-edge, dist)
	sphereRGB := clamp(col, 0.0, 1.0) * sphereA
	outRGB := sphereRGB + haloCol*(1.0-sphereA)
	outA := sphereA + halo*(1.0-sphereA)
	return vec4(clamp(outRGB, 0.0, 1.0), clamp(outA, 0.0, 1.0))
}
`)

var (
	orbCoreVar        = orbShader.Uniform("Core")
	orbMidVar         = orbShader.Uniform("Mid")
	orbRimVar         = orbShader.Uniform("Rim")
	orbPhaseVar       = orbShader.Uniform("Phase")
	orbEnergyVar      = orbShader.Uniform("Energy")
	orbRadiusRatioVar = orbShader.Uniform("RadiusRatio")
	orbSelectedVar    = orbShader.Uniform("Selected")
)

var frostShader = shader.MustCompile(`package main

var Tint vec4

func Fragment(dst vec4, pos vec2) vec4 {
	b := backdropSample(pos)
	tintPremul := vec4(Tint.rgb*Tint.a, Tint.a)
	res := tintPremul + b*(1.0-Tint.a)
	sheen := (1.0 - smoothstep(0.0, 2.0, pos.y)) * 0.06 * res.a
	res.rgb = clamp(res.rgb+vec3(sheen), 0.0, 1.0)
	return res
}
`)

var frostTintVar = frostShader.Uniform("Tint")

var voicePulseShader = shader.MustCompile(`package main

var Phase float
var Energy float
var Accent vec3

func Fragment(dst vec4, pos vec2) vec4 {
	size := rectSize()
	half := 0.5 * size
	s := min(size.x, size.y)
	uv := (pos - half) / max(0.5*s, 1.0)
	dist := length(uv)
	ang := atan2(uv.y, uv.x)

	env := smoothstep(1.0, 0.78, dist)
	if env <= 0.001 {
		return vec4(0.0)
	}

	rippleMod := 0.045 * (0.25 + 0.75*Energy) * (sin(ang*3.0+Phase*2.1) + 0.6*cos(ang*5.0-Phase*1.7))

	r0 := 0.22 + 0.05*sin(Phase*1.6)*Energy + 0.03*Energy
	core := smoothstep(r0+0.04, max(r0-0.02, 0.01), dist)

	rA := 0.36 + 0.03*sin(Phase*1.2) + rippleMod*0.60
	rB := 0.58 + 0.04*sin(Phase*1.2-1.1) + rippleMod*0.85
	rC := 0.80 + 0.05*sin(Phase*1.2-2.2) + rippleMod

	ringA := exp(-abs(dist-rA)*26.0) * (0.46 + 0.30*Energy)
	ringB := exp(-abs(dist-rB)*22.0) * (0.30 + 0.30*Energy)
	ringC := exp(-abs(dist-rC)*18.0) * (0.18 + 0.26*Energy)
	glow := exp(-dist*dist*6.2) * (0.16 + 0.18*Energy)

	alpha := clamp((core*0.30+ringA+ringB+ringC+glow)*env, 0.0, 1.0)
	col := mix(Accent, vec3(1.0), core*0.24+ringA*0.18)
	return vec4(clamp(col, 0.0, 1.0)*alpha, alpha)
}
`)

var (
	vpPhaseVar  = voicePulseShader.Uniform("Phase")
	vpEnergyVar = voicePulseShader.Uniform("Energy")
	vpAccentVar = voicePulseShader.Uniform("Accent")
)

// DrawShaderOrb renders a luminous glass Memory Orb via Kage shader with internal motif and optional selection halo.
func DrawShaderOrb(p *ui.Painter, r ui.Rect, theme domain.OrbTheme, weight float64, phase float32, energy float32, motif string, selected bool) {
	if p == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	baseDim := min(r.W, r.H)
	if baseDim <= 4 {
		return
	}

	palette := ColorsForTheme(theme)
	scaleFactor := float32(0.76 + 0.22*clamp01(weight))
	ratio := 0.88 * scaleFactor
	radius := 0.5 * baseDim * ratio

	selF := float32(0)
	if selected {
		selF = 1
	}
	if energy < 0 {
		energy = 0
	} else if energy > 1 {
		energy = 1
	}

	vals := orbShader.NewValues().
		Color(orbCoreVar, palette.Core).
		Color(orbMidVar, palette.Mid).
		Color(orbRimVar, palette.Rim).
		Float(orbPhaseVar, phase).
		Float(orbEnergyVar, energy).
		Float(orbRadiusRatioVar, ratio).
		Float(orbSelectedVar, selF)

	orbShader.Draw(p, r, shader.Options{Values: vals})

	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	if radius >= 7 {
		drawOrbMotif(p, cx, cy, radius*0.56, motif, ui.RGB(255, 255, 255).Alpha(0.48))
	}

	if selected {
		var selRing ui.Path
		selRing.Circle(cx, cy, radius+3.0)
		p.StrokePath(&selRing, 1.8, palette.Mid.Mix(palette.Rim, 0.45).Alpha(0.90))

		var innerRim ui.Path
		innerRim.Circle(cx, cy, radius)
		p.StrokePath(&innerRim, 1.2, ui.RGB(255, 255, 255).Alpha(0.72))
	}
}

// DrawFrostSurface paints a frosted backdrop-blurred glass surface with a 1px crisp rounded border stroke.
func DrawFrostSurface(p *ui.Painter, r ui.Rect, tint ui.Color, borderCol ui.Color, radius float32, blur float32) {
	if p == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	rad := min(radius, min(r.W, r.H)/2)
	if p.Preferences().ReduceTransparency || !p.Accelerated() {
		p.Fill(r, tint, rad)
		if r.W > rad*2 {
			sheen := ui.Rect{X: r.X + rad, Y: r.Y, W: r.W - rad*2, H: 1}
			p.Fill(sheen, ui.RGB(255, 255, 255).Alpha(36), 0)
		}
		if borderCol.A > 0 {
			p.Stroke(r, borderCol, rad, 1)
		}
		return
	}

	vals := frostShader.NewValues().Color(frostTintVar, tint)
	frostShader.Draw(p, r, shader.Options{
		BackdropBlur: blur,
		Radius:       rad,
		Values:       vals,
	})
	if borderCol.A > 0 {
		p.Stroke(r, borderCol, rad, 1)
	}
}

// DrawVoicePulse paints concentric breathing waveform ripples for the voice-first capture recorder.
func DrawVoicePulse(p *ui.Painter, r ui.Rect, phase float32, energy float32, accent ui.Color) {
	if p == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	if min(r.W, r.H) <= 4 {
		return
	}
	if energy < 0 {
		energy = 0
	} else if energy > 1 {
		energy = 1
	}
	vals := voicePulseShader.NewValues().
		Float(vpPhaseVar, phase).
		Float(vpEnergyVar, energy).
		Color(vpAccentVar, accent)
	voicePulseShader.Draw(p, r, shader.Options{Values: vals})
}
