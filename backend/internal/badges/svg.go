// Package badges implements Uptime Kuma's dynamic SVG status badge feature:
// public, embeddable badge images for a device's live status/uptime/ping/
// response-time/cert-expiry, meant to be dropped into a README or wiki page
// (e.g. `![status](https://host/api/v1/badge/123/status)`).
//
// Kuma renders these with the `badge-maker` npm package, which isn't
// available in Go, so this file hand-rolls ONE reasonable badge style —
// shields.io's classic "flat" look (two rounded rects, label + message) —
// rather than reimplementing all four styles badge-maker supports
// (flat/flat-square/plastic/for-the-badge). The `style` query parameter is
// accepted (so existing Kuma badge URLs with `?style=plastic` etc. don't
// 404) but every style renders identically today.
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

// Render renders a shields.io-"flat"-style badge: a grey label box on the
// left, a colored message box on the right, both with slightly rounded
// left/right corners respectively, and a subtle top-highlight gradient the
// same way shields.io's flat style does.
func Render(label, message, color string) string {
	if strings.TrimSpace(color) == "" {
		color = ColorInfo
	}
	const pad = 10.0    // horizontal padding inside each box
	const height = 20.0 // shields.io flat badges are 20px tall
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

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" role="img" aria-label="%s: %s">
<title>%s: %s</title>
<linearGradient id="s" x2="0" y2="100%%">
<stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
<stop offset="1" stop-opacity=".1"/>
</linearGradient>
<clipPath id="r">
<rect width="%.0f" height="%.0f" rx="3" fill="#fff"/>
</clipPath>
<g clip-path="url(#r)">
<rect width="%.1f" height="%.0f" fill="#555"/>
<rect x="%.1f" width="%.1f" height="%.0f" fill="%s"/>
<rect width="%.0f" height="%.0f" fill="url(#s)"/>
</g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
<text x="%.1f" y="14" fill="#010101" fill-opacity=".3">%s</text>
<text x="%.1f" y="13">%s</text>
<text x="%.1f" y="14" fill="#010101" fill-opacity=".3">%s</text>
<text x="%.1f" y="13">%s</text>
</g>
</svg>`,
		totalW, height, escapeXML(label), escapeXML(message),
		escapeXML(label), escapeXML(message),
		totalW, height,
		labelW, height,
		labelW, msgW, height, color,
		totalW, height,
		labelX, escapeXML(label),
		labelX, escapeXML(label),
		msgX, escapeXML(message),
		msgX, escapeXML(message),
	)
}
