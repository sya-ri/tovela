package locale

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"
)

//go:embed translations/*.json
var catalogs embed.FS

var (
	language = "en"
	messages map[string]string
)

// Language returns the language of the active Tovela catalog.
func Language() string { return language }

// Use selects the app language before any windows or relay services are started.
func Use(tag string) error {
	base, _, _ := strings.Cut(strings.ToLower(tag), "-")
	data, err := catalogs.ReadFile("translations/" + base + ".json")
	if errors.Is(err, fs.ErrNotExist) {
		language, messages = "en", nil
		return nil
	}
	if err != nil {
		return err
	}
	var translated map[string]string
	if err := json.Unmarshal(data, &translated); err != nil {
		return err
	}
	language, messages = base, translated
	return nil
}

// T translates an app message, with its English source as the fallback.
func T(source string) string {
	if translated := messages[source]; translated != "" {
		return translated
	}
	return source
}
