---
name: tovela-desktop-ui
description: Review and implement Tovela's Go/Fyne desktop screens, spacing, keyboard interactions, loading and error states, and visual snapshots.
license: MIT
---

# Tovela desktop UI

Use the existing Fyne controls and theme so that focus, disabled states, and platform color modes work consistently. Keep the relay status, hostname, and next action easy to find. Use short action labels and omit copy that repeats what the control shows.

- Start with [Fyne widgets](https://docs.fyne.io/architecture/widgets/) and [layouts](https://docs.fyne.io/explore/layouts/). Use theme padding between controls and between button icons and labels. Use the same button sizes for related actions. Allow extra space for a wide brand mark without stretching it.
- Group controls by purpose. Keep one clear primary action for the current state, including when a new profile needs setup. Match the [Windows layout guidance](https://learn.microsoft.com/windows/apps/design/basics/content-basics) and [Apple button guidance](https://developer.apple.com/design/human-interface-guidelines/buttons) where they apply.
- Preserve visible focus, natural Tab order, and the standard activation keys. Test with the keyboard using the [GNOME keyboard guidance](https://developer.gnome.org/hig/guidelines/keyboard.html). Keep text labels on actions whose icons are ambiguous.
- Keep cloud requests outside the UI thread. Show loading and errors near the corresponding controls and let the user continue when some locations cannot be measured. Label regional connection measurements in milliseconds and reuse them for the location recommendation.
- Use [Fyne visual and interaction tests](https://docs.fyne.io/started/testing/) through `mise exec -- go test -tags ci ./...`. Review English and Japanese at the default and a narrow window size. Check light and dark themes and long hostnames or locations when the change can affect them. Use mocked providers and test data in snapshots.
