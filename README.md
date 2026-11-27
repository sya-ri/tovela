<h1 align="center">
  <img src="docs/brand/logo.svg" alt="Tovela" width="360">
</h1>

Tovela is a desktop app for sharing local Minecraft Java servers through temporary Google Compute Engine (GCE) relays. Choose a relay location for each hostname or let the app pick by connection time. Each relay supports an optional UDP port. Start, extend, or stop each relay from Windows, macOS, or Linux. The interface is available in English and Japanese.

## Requirements

Run a local Minecraft Java server and bring a Google Cloud project with billing and a Cloudflare-managed domain. The app bundles frp. See the [design](docs/design.md) and [development notes](docs/development.md).

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

![Estimated monthly costs for Iowa and Tokyo compared with Playit Premium](docs/monthly-relay-cost.svg)

At Playit Premium’s $3/month starting price, the example break-even points are **215 h** in Iowa (`us-central1`) and **182 h** in Tokyo (`asia-northeast1`). Costs vary by region. [Playit pricing](https://playit.gg/)

Estimates cover an `e2-micro`, a 10 GiB standard disk, and an IPv4 address while allocated. They exclude transfer, taxes, domain fees, and credits. Eligible `e2-micro` usage in some US regions qualifies for the [Google Cloud Free Tier](https://docs.cloud.google.com/free/docs/free-cloud-features#compute). [Compute pricing](https://cloud.google.com/products/compute/pricing/general-purpose), [disk pricing](https://cloud.google.com/compute/disks-image-pricing), and [IPv4 pricing](https://cloud.google.com/vpc/network-pricing#ipaddress).
