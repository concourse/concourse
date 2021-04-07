package docs

import (
	"math"

	"github.com/lucasb-eyer/go-colorful"
)

var white = colorful.Color{
	R: 1,
	B: 1,
	G: 1,
}

var black = colorful.Color{
	R: 0,
	G: 0,
	B: 0,
}

// red, green, and blue coefficients
const rc = 0.2126
const gc = 0.7152
const bc = 0.0722

// low-gamma adjust coefficient
const lowc = 1 / 12.92

func adjustGamma(v float64) float64 {
	return math.Pow((v+0.055)/1.055, 2.4)
}

func contrast(a, b colorful.Color) float64 {
	al := relativeLuminance(a)
	bl := relativeLuminance(b)
	lighter := math.Max(al, bl)
	darker := math.Min(al, bl)
	return (lighter + 0.05) / (darker + 0.05)
}

func relativeLuminance(color colorful.Color) float64 {
	var r, g, b float64

	if color.R <= 0.03928 {
		r = color.R * lowc
	} else {
		r = adjustGamma(color.R)
	}

	if color.G <= 0.03928 {
		g = color.G * lowc
	} else {
		g = adjustGamma(color.G)
	}

	if color.B <= 0.03928 {
		b = color.B * lowc
	} else {
		b = adjustGamma(color.B)
	}

	return r*rc + g*gc + b*bc
}
