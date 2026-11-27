# Getting started

Tovela needs a local Minecraft Java server, a Google Cloud project with billing, and a domain managed in Cloudflare.

## Set up Google Cloud

1. Enable the Compute Engine API. The signed-in user needs to create and delete instances and boot disks, attach an external IPv4 address, and update VM scheduling. On a standard project with its own VPC, start with the [Compute Instance Admin role](https://docs.cloud.google.com/compute/docs/access/iam#compute.instanceAdmin.v1).
2. In the VPC you will select in Tovela, allow these inbound ports on instances tagged `tovela`. Restrict TCP 7000 to your app's outbound IP when it is stable. [VPC firewall rules](https://docs.cloud.google.com/firewall/docs/using-firewalls)

   | Port | Used by |
   | --- | --- |
   | TCP 7000 | The desktop app connects to frp |
   | TCP 25565 | Players connect to Minecraft Java |
   | Your UDP port | Clients connect to your optional UDP service |

3. Create a Google OAuth client of type **Desktop app**. Add the operator as a test user if the consent screen is in testing. [Google desktop OAuth](https://developers.google.com/identity/protocols/oauth2/native-app)

## Set up Cloudflare

Create an OAuth client with the settings below. A private client is sufficient for members of the Cloudflare account. [Cloudflare OAuth clients](https://developers.cloudflare.com/fundamentals/oauth/create-an-oauth-client/)

| Setting | Value |
| --- | --- |
| Grants | Authorization code and refresh token |
| Token authentication | None |
| Redirect URI | `http://127.0.0.1:42813/callback` |
| Permissions | Zone Read and DNS Write |

## Start a relay

1. Start Minecraft Java locally. If you want to forward UDP, start that service too.
2. Open **Settings** in Tovela. Enter your Google Cloud project, Tokyo zone, VPC, Cloudflare zone, public hostname, and local ports. Enter the OAuth client IDs and sign in to both providers. The app stores tokens in the OS credential store.
3. Select **Start**. When the relay is **Online**, enter your hostname in Minecraft. Java uses TCP 25565 publicly, even if you use a different local TCP port. UDP uses the port you entered.
4. Select **Extend 4 h** to add time or **Stop** to remove the DNS record and delete the VM and boot disk. Keep Tovela and the local server running while players are connected.

Use **Add relay profile** to publish another hostname. Give each local server a different TCP port. Each active profile runs and bills its own VM.

> [!IMPORTANT]
> **Replace existing DNS records** removes conflicts of type A, AAAA, CNAME, or Java SRV once the relay is ready. **Stop** removes Tovela's A record. Restore any old records manually if needed. DNS caches can outlast a change.

## Build from source

Use the versions in [mise.toml](../mise.toml). Build on the target OS and CPU so the script bundles the matching frp client. Windows needs WebView2. Linux needs GTK4, WebKitGTK 6.0, and a running Secret Service provider for credentials.

```sh
pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend build
node scripts/bundle-frp.mjs
go build -tags production -o build/tovela .
```

On Windows, use `-o build/Tovela.exe`. The [desktop workflow](../.github/workflows/desktop.yml) shows how the platform packages are built.

## Tests

Build the frontend and bundle frp first. Then run the Go tests and the Chromium UI snapshots.

```sh
go test ./...
pnpm --dir frontend exec playwright install chromium
pnpm --dir frontend test
```
