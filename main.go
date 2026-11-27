package main

import (
	_ "embed"
	"errors"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/lang"
	"github.com/gofrs/flock"
	"github.com/sya-ri/tovela/internal/desktop"
	"github.com/sya-ri/tovela/internal/locale"
	"github.com/sya-ri/tovela/internal/relay"
)

//go:embed docs/brand/icon.svg
var icon []byte

var googleOAuthClientID, cloudflareOAuthClientID string

func main() {
	a := app.NewWithID("dev.sya-ri.tovela")
	if err := locale.Use(lang.SystemLocale().LanguageString()); err != nil {
		log.Print(err)
	}
	a.SetIcon(fyne.NewStaticResource("icon.svg", icon))
	window := a.NewWindow("Tovela")
	window.Resize(fyne.NewSize(610, 500))

	lock, err := lockInstance()
	if err != nil {
		d := dialog.NewError(err, window)
		d.SetOnClosed(a.Quit)
		d.Show()
		window.ShowAndRun()
		return
	}
	defer func() {
		if err := lock.Close(); err != nil {
			log.Print(err)
		}
	}()

	service, err := relay.NewDesktop(googleOAuthClientID, cloudflareOAuthClientID)
	if err != nil {
		d := dialog.NewError(err, window)
		d.SetOnClosed(a.Quit)
		d.Show()
		window.ShowAndRun()
		return
	}
	a.Settings().SetTheme(desktop.Theme())
	desktop.Run(a, window, service)
}

func lockInstance() (*flock.Flock, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(configDir, "tovela")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(dir, "app.lock"))
	ok, err := lock.TryLock()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New(locale.T("Tovela is already open."))
	}
	return lock, nil
}
