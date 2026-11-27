<h1 align="center">
  <img src="docs/brand/logo.svg" alt="Tovela" width="360">
</h1>

Tovela is a desktop app for sharing local Minecraft Java servers through temporary Google Compute Engine (GCE) relays. Each relay supports a custom hostname and an optional UDP port. Start, extend, or stop each relay from Windows, macOS, or Linux.

## Getting started

Tovela is in development. A relay requires a Google Cloud project with billing and a domain managed in Cloudflare. See the [design](docs/design.md) for how the relay works.

## Service comparison

✅ Supported, ⚠️ limited, ❌ unavailable. Prices checked November 27, 2026.

| Service | Price | Stable endpoint | Custom hostname | Java list ping on custom hostname | UDP |
| --- | --- | --- | --- | --- | --- |
| [ngrok Free](https://ngrok.com/pricing) | $0 (card verification for TCP) | ❌ Random hostname and port | ⚠️ DNS alias needs updates | ✅ Raw TCP via DNS alias | ❌ [TCP only](https://ngrok.com/compare/tailscale) |
| [ngrok Hobbyist](https://ngrok.com/docs/gateway/domains/tcp-addresses) | $10/month | ✅ Reserved TCP address | ⚠️ [CNAME with assigned port](https://ngrok.com/docs/gateway/domains/custom-domains#using-custom-domains-with-tcp-endpoints) or Java SRV | ✅ Raw TCP via DNS alias | ❌ |
| [Playit Free](https://playit.gg/) | $0 | ✅ Fixed for the tunnel’s lifetime | ⚠️ Java SRV workaround | ⚠️ Failed in one SRV test | ✅ Separate UDP tunnels |
| [Playit Premium](https://playit.gg/support/playit-premium/) | From $3/month | ✅ Assigned hostname and port | ✅ [External domains](https://playit.gg/support/external-domain-cloudflare/) | ⚠️ Provider example timed out | ✅ Combined TCP+UDP |
| Tovela | From $0 | ✅ Fixed hostname, ephemeral IP | ✅ Cloudflare DNS, direct TCP 25565 | ✅ Supported | ✅ TCP and optional UDP |

ngrok documents [raw TCP forwarding for Minecraft and CNAME aliases](https://ngrok.com/docs/gateway/endpoints/tcp). Native HTTP/TLS domains start at $20/month. [ngrok pricing](https://ngrok.com/pricing)

The Playit checks covered one free Java 1.21.11 tunnel and the [Premium example](https://playit.gg/support/external-domain-cloudflare/).

### Monthly relay cost

![Relay cost reaches Playit Premium’s $3/month starting price at about 182 hours in Tokyo and 215 hours in Iowa.](docs/monthly-relay-cost.svg)

At Playit Premium’s $3/month starting price, the break-even points are **182 h** in Tokyo (`asia-northeast1`) and **215 h** in Iowa (`us-central1`). Tovela uses Tokyo. Iowa is Google’s pricing table default. [Playit pricing](https://playit.gg/)

Rates are about $0.0165/h in Tokyo and $0.0139/h in Iowa per relay. They cover an `e2-micro`, a 10 GiB standard disk, and IPv4 while allocated. Transfer, tax, domain fees, and credits are separate. [Compute pricing](https://cloud.google.com/products/compute/pricing/general-purpose), [disk pricing](https://cloud.google.com/compute/disks-image-pricing), and [IPv4 pricing](https://cloud.google.com/vpc/network-pricing#ipaddress).
