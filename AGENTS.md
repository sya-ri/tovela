# Tovela

Write documentation, commit messages, pull requests, and code comments in English. Keep desktop text available in English and Japanese.

Use the [design](docs/design.md) for the relay contract and the [development guide](docs/development.md) for build and cloud requirements. Verify changing provider flags, scopes, and fields in the official documentation.

## Relay

- Keep provider calls cancellable with contexts. Set deadlines for network and subprocess operations and reconcile partial state on retry. [Google Go Style](https://google.github.io/styleguide/go/)
- Reuse the request ID when retrying a Compute Engine create. During recovery and deletion, check the deterministic instance name, the instance ID when available, and the ownership labels. Set a server-side deletion deadline. [Instance creation](https://docs.cloud.google.com/compute/docs/reference/rest/v1/instances/insert), [automatic termination](https://docs.cloud.google.com/compute/docs/instances/limit-vm-runtime)
- Pin frpc and frps to the same version and verify the release assets. Authenticate the control connection and verify the server's TLS identity with an explicit trust configuration. [frp authentication](https://gofrp.org/en/docs/features/common/authentication/), [frp TLS](https://gofrp.org/en/docs/features/common/network/network-tls/)
- Publish a DNS-only record after the relay is ready and check ownership before cleanup. Use the Cloudflare batch API to replace records after obtaining consent. [Cloudflare DNS batch API](https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/batch/)
- Use OAuth and disposable inputs for verification. Keep credentials, game data, and deployment-specific settings out of commits and snapshots. Verify cloud resource deletion after a live test.

## Desktop

- Use existing Fyne controls and theme padding. Keep the relay status, hostname, and primary action easy to find. Use short labels, consistent button sizes, and enough space for wide provider marks without stretching them. [Fyne widgets](https://docs.fyne.io/architecture/widgets/), [Fyne layouts](https://docs.fyne.io/explore/layouts/)
- Left-align Settings section headings and field labels. Align controls in the right column and keep account and save actions in the scrollable form. Use more space between sections than between fields. [Windows app settings](https://learn.microsoft.com/windows/apps/design/app-settings/guidelines-for-app-settings), [GNOME preference groups](https://gnome.pages.gitlab.gnome.org/libadwaita/doc/1.3/class.PreferencesGroup.html)
- Preserve visible focus, natural Tab order, and standard activation keys. Label actions whose icons are ambiguous. [GNOME keyboard guidance](https://developer.gnome.org/hig/guidelines/keyboard.html)
- Keep cloud requests off the UI thread. Show loading and errors near the affected controls. Use regional connection times in milliseconds for display and recommendations. Allow manual selection when measurements are unavailable.
- Run Fyne interaction and snapshot tests with `mise exec -- go test -tags ci ./...`. Check English and Japanese at the default and narrow window sizes. Include light and dark themes and long hostnames or locations when relevant. Use mock providers for snapshots. [Fyne testing](https://docs.fyne.io/started/testing/)