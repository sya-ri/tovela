package locale

import (
	"fmt"
	"testing"
)

func TestLanguageSelection(t *testing.T) {
	for _, tc := range []struct {
		tag            string
		button         string
		transportError string
	}{
		{"en-US", "Open Protection History", "Enter the Google OAuth client ID."},
		{"ja-JP", "保護の履歴を開く", "Google の OAuth クライアント ID を入力してください。"},
		{"fr-FR", "Open Protection History", "Enter the Google OAuth client ID."},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			if err := Use(tc.tag); err != nil {
				t.Fatal(err)
			}
			if button := T("Open Protection History"); button != tc.button {
				t.Errorf("button = %q, want %q", button, tc.button)
			}
			if message := fmt.Sprintf(T("Enter the %s OAuth client ID."), "Google"); message != tc.transportError {
				t.Errorf("message = %q, want %q", message, tc.transportError)
			}
		})
	}
}
