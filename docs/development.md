# Development

[mise.toml](../mise.toml) pins Go. Build on the target OS and CPU with the [Fyne build dependencies](https://docs.fyne.io/started/quick/). The [desktop workflow](../.github/workflows/desktop.yml) builds the distribution for each OS.

```sh
go run ./scripts
go test -tags ci ./...
go build -o build/tovela .
```

The tests use the software renderer to compare snapshots without opening a window.

## Cloud requirements

- Google Cloud needs the [Compute Engine API](https://docs.cloud.google.com/compute/docs/access/iam) and permission to list and read machine types (`compute.machineTypes.list` and `compute.machineTypes.get`) and to create, update, and delete instances and boot disks with external IPv4 addresses. On the VPC, allow TCP 7000, TCP 25565, and the requested UDP port as needed on instances tagged `tovela`. [Firewall rules](https://docs.cloud.google.com/firewall/docs/using-firewalls)
- A [Google desktop OAuth client](https://developers.google.com/identity/protocols/oauth2/native-app) and a [Cloudflare OAuth client](https://developers.cloudflare.com/fundamentals/oauth/create-an-oauth-client/) let the operator sign in. The Cloudflare client uses authorization code and refresh token grants, public token authentication, Zone Read and DNS Write, and the redirect URI `http://127.0.0.1:42813/callback`. Supply public IDs through the workflow variables `GOOGLE_OAUTH_CLIENT_ID` and `CLOUDFLARE_OAUTH_CLIENT_ID` for distribution, or enter them under **Advanced** in the app.
- Linux needs a running Secret Service provider for credentials.

**Replace existing DNS records** deletes conflicting records for the selected hostname. Stopping removes the relay's record. Restore any previous records separately.

Login buttons use provider marks from [Google Identity](https://developers.google.com/identity/branding-guidelines) and [Cloudflare Developers](https://developers.cloudflare.com/favicon.png).

The Japanese interface bundles [M PLUS 1p](https://github.com/google/fonts/tree/d714b17ce2379f06daf6295617f961df605dccb5/ofl/mplus1p). Distributions include its [Open Font License](../internal/desktop/fonts/OFL.txt).
