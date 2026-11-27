package desktop

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icons/google.png
var googleIconPNG []byte

//go:embed icons/cloudflare.png
var cloudflareIconPNG []byte

var (
	googleIcon     = fyne.NewStaticResource("google.png", googleIconPNG)
	cloudflareIcon = fyne.NewStaticResource("cloudflare.png", cloudflareIconPNG)
)
