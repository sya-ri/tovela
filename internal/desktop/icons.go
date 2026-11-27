package desktop

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed icons/google.png
var googleIconPNG []byte

//go:embed icons/cloudflare.svg
var cloudflareIconSVG []byte

var (
	googleIcon     = fyne.NewStaticResource("google.png", googleIconPNG)
	cloudflareIcon = fyne.NewStaticResource("cloudflare.svg", cloudflareIconSVG)
)

// Give the wider mark more room while keeping the original proportions.
type cloudflareButtonTheme struct{ fyne.Theme }

func (t cloudflareButtonTheme) Size(name fyne.ThemeSizeName) float32 {
	size := t.Theme.Size(name)
	if name == theme.SizeNameInlineIcon {
		return size * 1.4
	}
	return size
}
