package desktop

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/sya-ri/tovela/internal/locale"
)

//go:embed fonts/regular.ttf
var mplusRegular []byte

//go:embed fonts/bold.ttf
var mplusBold []byte

var (
	japaneseRegular = fyne.NewStaticResource("MPLUS1p-Regular.ttf", mplusRegular)
	japaneseBold    = fyne.NewStaticResource("MPLUS1p-Bold.ttf", mplusBold)
)

type palette struct {
	fyne.Theme
	useJapaneseFont bool
}

// Theme applies the relay palette and retains Fyne's platform theme variant.
func Theme() fyne.Theme {
	return palette{theme.DefaultTheme(), locale.Language() == "ja"}
}

func (p palette) Font(style fyne.TextStyle) fyne.Resource {
	if !p.useJapaneseFont || style.Monospace || style.Symbol {
		return p.Theme.Font(style)
	}
	if style.Bold {
		return japaneseBold
	}
	return japaneseRegular
}

func (p palette) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if variant == theme.VariantDark {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 28, G: 27, B: 42, A: 255}
		case theme.ColorNameInputBackground:
			return color.NRGBA{R: 48, G: 46, B: 69, A: 255}
		case theme.ColorNamePrimary:
			return color.NRGBA{R: 187, G: 178, B: 246, A: 255}
		case theme.ColorNameForegroundOnPrimary:
			return color.NRGBA{R: 28, G: 27, B: 42, A: 255}
		case theme.ColorNameSuccess:
			return color.NRGBA{R: 148, G: 218, B: 176, A: 255}
		}
	} else {
		switch name {
		case theme.ColorNameBackground:
			return color.NRGBA{R: 242, G: 241, B: 247, A: 255}
		case theme.ColorNameInputBackground:
			return color.NRGBA{R: 255, G: 253, B: 255, A: 255}
		case theme.ColorNamePrimary:
			return color.NRGBA{R: 55, G: 54, B: 87, A: 255}
		case theme.ColorNameForeground:
			return color.NRGBA{R: 38, G: 36, B: 59, A: 255}
		case theme.ColorNameSuccess:
			return color.NRGBA{R: 36, G: 88, B: 58, A: 255}
		case theme.ColorNameWarning:
			return color.NRGBA{R: 127, G: 76, B: 15, A: 255}
		}
	}
	return p.Theme.Color(name, variant)
}
