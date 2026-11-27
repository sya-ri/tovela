package desktop

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icons/google.png
var googleIconPNG []byte

//go:embed icons/cloudflare.svg
var cloudflareIconSVG []byte

var (
	googleIcon     = fyne.NewStaticResource("google.png", googleIconPNG)
	cloudflareIcon = fyne.NewStaticResource("cloudflare.svg", cloudflareIconSVG)
)
