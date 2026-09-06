// Package badges implements Uptime Kuma's dynamic SVG status badge feature:
// public, embeddable badge images for a device's live status/uptime/ping/
// response-time/cert-expiry, meant to be dropped into a README or wiki page
// (e.g. `![status](https://host/api/v1/badge/123/status)`).
//
// Kuma renders these with the `badge-maker` npm package, which isn't
// available in Go, so this file hand-rolls the four visual styles
// badge-maker supports: "flat" (default, two rounded rects with a subtle
// top-highlight gradient), "flat-square" (same two-box layout, square
// corners, no gradient), "plastic" (rounded corners, a stronger two-stop
// gloss gradient), and "for-the-badge" (bold, taller, all-caps, wide
// letter-spacing, square corners) — matching shields.io's own visual
// distinctions between these styles closely enough for embedding purposes,
// without pulling in badge-maker itself. An unrecognized `style` value
// falls back to "flat", same as shields.io.
package badges

import (
	"fmt"
	"strings"
)

// naColor is shown whenever the requested device isn't visible on any
// published public status page, matching Kuma's badgeConstants.naColor
// behavior: never leak whether a private/nonexistent device exists, just
// show a neutral grey "N/A".
const naColor = "#9f9f9f"

// Reasonable flat-badge default colors, matching shields.io's usual palette
// (Kuma itself just forwards whatever badge-maker's defaults are; these are
// the same family of colors).
const (
	ColorUp          = "#4c1"
	ColorDown        = "#e05d44"
	ColorPending     = "#dfb317"
	ColorMaintenance = "#1e90ff"
	ColorPaused      = "#9f9f9f"
	ColorNA          = naColor
	ColorInfo        = "#007ec6"
)

// charWidth is a rough per-character pixel-width heuristic for the default
// SVG sans-serif font at 11px — not real font metrics, just enough so
// labels/messages of different lengths don't visibly overlap or leave huge
// gaps. Narrow characters (i, l, punctuation, space) get less width, wide
// characters (m, w, uppercase) get more, everything else averages out.
func charWidth(r rune) float64 {
	switch {
	case r == ' ' || r == '.' || r == ':' || r == 'i' || r == 'l' || r == 'I' || r == '\'':
		return 4
	case r == 'm' || r == 'M' || r == 'w' || r == 'W' || r == '%':
		return 10
	case r >= 'A' && r <= 'Z':
		return 7.5
	case r >= '0' && r <= '9':
		return 6.5
	default:
		return 6
	}
}

func textWidth(s string) float64 {
	w := 0.0
	for _, r := range s {
		w += charWidth(r)
	}
	return w
}

func escapeXML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// Render renders a badge in the given shields.io style ("flat" [default],
// "flat-square", "plastic", or "for-the-badge"); any other/empty value
// falls back to "flat".
func Render(label, message, color, style string) string {
	if strings.TrimSpace(color) == "" {
		color = ColorInfo
	}
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "for-the-badge":
		return renderForTheBadge(label, message, color)
	case "plastic":
		return renderTwoBox(label, message, color, 4, true)
	case "flat-square":
		return renderTwoBox(label, message, color, 0, false)
	default: // "flat" and anything unrecognized
		return renderTwoBox(label, message, color, 3, true)
	}
}

// renderTwoBox renders the classic shields.io two-box layout shared by
// "flat", "flat-square" and "plastic": a grey label box on the left, a
// colored message box on the right. `gradient` adds the subtle top-highlight
// gloss ("flat" and "plastic" have it, "flat-square" doesn't); `rx` is the
// outer corner radius (0 for "flat-square"'s sharp corners, slightly more
// for "plastic" than "flat" to read as more rounded/glossy).
func renderTwoBox(label, message, color string, rx float64, gradient bool) string {
	const pad = 10.0
	const height = 20.0
	labelW := textWidth(label) + pad*2
	msgW := textWidth(message) + pad*2
	if labelW < 24 {
		labelW = 24
	}
	if msgW < 24 {
		msgW = 24
	}
	totalW := labelW + msgW
	labelX := labelW / 2
	msgX := labelW + msgW/2
	gradientDef := ""
	gradientRect := ""
	if gradient {
		gradientDef = `<linearGradient id="s" x2="0" y2="100%">
<stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
<stop offset="1" stop-opacity=".1"/>
</linearGradient>
`
		gradientRect = fmt.Sprintf(`<rect width="%.0f" height="%.0f" fill="url(#s)"/>
`, totalW, height)
	}

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" role="img" aria-label="%s: %s">
<title>%s: %s</title>
%s<clipPath id="r">
<rect width="%.0f" height="%.0f" rx="%.0f" fill="#fff"/>
</clipPath>
<g clip-path="url(#r)">
<rect width="%.1f" height="%.0f" fill="#555"/>
<rect x="%.1f" width="%.1f" height="%.0f" fill="%s"/>
%s</g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
<text x="%.1f" y="14" fill="#010101" fill-opacity=".3">%s</text>
<text x="%.1f" y="13">%s</text>
<text x="%.1f" y="14" fill="#010101" fill-opacity=".3">%s</text>
<text x="%.1f" y="13">%s</text>
</g>
</svg>`,
		totalW, height, escapeXML(label), escapeXML(message),
		escapeXML(label), escapeXML(message),
		gradientDef,
		totalW, height, rx,
		labelW, height,
		labelW, msgW, height, color,
		gradientRect,
		labelX, escapeXML(label),
		labelX, escapeXML(label),
		msgX, escapeXML(message),
		msgX, escapeXML(message),
	)
}

// renderForTheBadge renders shields.io's "for-the-badge" style: taller
// (28px), square corners, bold uppercase text with wide letter-spacing, and
// wider padding -- visually the most distinct of the four styles.
func renderForTheBadge(label, message, color string) string {
	const pad = 12.0
	const height = 28.0
	upperLabel := strings.ToUpper(label)
	upperMsg := strings.ToUpper(message)
	// for-the-badge's letter-spacing widens each glyph; approximate with a
	// small per-character bonus on top of the normal width heuristic.
	labelW := textWidth(upperLabel) + float64(len([]rune(upperLabel)))*1.2 + pad*2
	msgW := textWidth(upperMsg) + float64(len([]rune(upperMsg)))*1.2 + pad*2
	if labelW < 40 {
		labelW = 40
	}
	if msgW < 40 {
		msgW = 40
	}
	totalW := labelW + msgW
	labelX := labelW / 2
	msgX := labelW + msgW/2

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" role="img" aria-label="%s: %s">
<title>%s: %s</title>
<g shape-rendering="crispEdges">
<rect width="%.1f" height="%.0f" fill="#555"/>
<rect x="%.1f" width="%.1f" height="%.0f" fill="%s"/>
</g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="10" font-weight="bold" letter-spacing="1">
<text x="%.1f" y="18">%s</text>
<text x="%.1f" y="18">%s</text>
</g>
</svg>`,
		totalW, height, escapeXML(label), escapeXML(message),
		escapeXML(label), escapeXML(message),
		labelW, height,
		labelW, msgW, height, color,
		labelX, escapeXML(upperLabel),
		msgX, escapeXML(upperMsg),
	)
}
