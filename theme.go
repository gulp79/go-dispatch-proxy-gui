package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// MatrixTheme è un tema personalizzato cross-platform
type MatrixTheme struct{}

var _ fyne.Theme = (*MatrixTheme)(nil)

func (m MatrixTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	// Log area verde Matrix
	case theme.ColorNameDisabled:
		return color.RGBA{0, 255, 65, 255} // Verde fosforescente
	case theme.ColorNameDisabledButton:
		return color.RGBA{0, 180, 45, 255}
	
	// Sfondo log nero profondo
	case theme.ColorNameInputBackground:
		return color.RGBA{10, 10, 10, 255}
	
	// ✓ CORREZIONI LINUX - Forza tema scuro uniforme
	case theme.ColorNameBackground:
		return color.RGBA{30, 30, 30, 255} // Background principale
	case theme.ColorNameButton:
		return color.RGBA{60, 60, 60, 255} // Bottoni
	case theme.ColorNameForeground:
		return color.RGBA{240, 240, 240, 255} // Testo principale
	case theme.ColorNameHover:
		return color.RGBA{80, 80, 80, 255} // Hover
	case theme.ColorNameShadow:
		return color.RGBA{0, 0, 0, 100} // Ombre
	
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m MatrixTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m MatrixTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m MatrixTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
