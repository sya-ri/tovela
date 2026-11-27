<h1 align="center">
  <img src="docs/brand/logo.svg" alt="Tovela" width="360">
</h1>

Share a local Minecraft Java server or another multiplayer game server on your own domain. Tovela starts an on-demand cloud relay and forwards TCP and UDP to your computer. Forward multiple ports on one hostname for a game server, a plugin's web page, or other local services. Start, extend, or stop relays from Windows, macOS, or Linux. The interface is available in English and Japanese.

## Requirements

Run the server on the same computer as Tovela. Bring a Google Cloud project with billing and a Cloudflare-managed domain. See the [design](docs/design.md) and [development notes](docs/development.md).

## Default game server ports

A new profile starts with the **Minecraft** mapping for Java Edition. Match your server settings with one **TCP** or **UDP** mapping per port and protocol.

| Game | Default ports | Server reference |
| --- | --- | --- |
| Minecraft Java Edition | TCP 25565 | [Minecraft Help](https://help.minecraft.net/hc/en-us/articles/360058525452) |
| Minecraft Bedrock Edition (NetherNet) | TCP 19132 for signaling, UDP as configured | [Minecraft Help](https://help.minecraft.net/hc/en-us/articles/49914677714957) |
| 7 Days to Die | TCP 26900, UDP 26900–26903 | [7 Days to Die Help Center](https://support.7daystodie.com/hc/en-us/articles/48451426400916-Opening-Ports-in-Firewall) |
| BeamMP | TCP and UDP 30814 | [BeamMP Docs](https://docs.beammp.com/server/port-forwarding/) |
| Factorio | UDP 34197 | [Factorio Wiki](https://wiki.factorio.com/Multiplayer) |
| Hytale | UDP 5520 | [Hytale Server Manual](https://support.hytale.com/hc/en-us/articles/45326769420827-Hytale-Server-Manual) |
| Palworld | UDP 8211 | [Palworld Server Guide](https://docs.palworldgame.com/getting-started/requirements/) |
| Project Zomboid (Steam) | UDP 16261 and 16262 | [The Indie Stone](https://theindiestone.com/forums/topic/60581-4177-released/) |
| Rust | UDP 28015 for direct connections | [Facepunch Wiki](https://wiki.facepunch.com/rust/creating-a-server) |
| Terraria | TCP 7777 | [Terraria Wiki](https://terraria.wiki.gg/wiki/Server#Before_You_Host) |
| Unturned | UDP 27015 and 27016 | [Unturned Docs](https://docs.smartlydressedgames.com/en/latest/servers/port-forwarding.html) |
| Valheim (Steam backend) | UDP 2456 and 2457 | [Valheim Server Guide](https://www.valheimgame.com/support/a-guide-to-dedicated-servers/) |
| Vintage Story | TCP 42420 | [Vintage Story Wiki](https://wiki.vintagestory.at/Setting_up_a_Multiplayer_Server/en) |

## Service comparison

✅ Supported, ⚠️ limited or inferred, ❌ unavailable. Prices checked November 27, 2026.

| Service | Price | Stable endpoint | Custom hostname | Minecraft Java list ping on custom hostname | UDP |
| --- | --- | --- | --- | --- | --- |
| [ngrok Free](https://ngrok.com/pricing) | $0 (card verification for TCP) | ❌ Random hostname and port | ⚠️ DNS alias needs updates | ✅ Raw TCP via DNS alias | ❌ [TCP only](https://ngrok.com/compare/tailscale) |
| [ngrok Hobbyist](https://ngrok.com/docs/gateway/domains/tcp-addresses) | $10/month | ✅ Reserved TCP address | ⚠️ [CNAME with assigned port](https://ngrok.com/docs/gateway/domains/custom-domains#using-custom-domains-with-tcp-endpoints) or Java SRV | ✅ Raw TCP via DNS alias | ❌ |
| [Playit Free](https://playit.gg/) | $0 | ✅ Fixed for the tunnel’s lifetime | ⚠️ Java SRV workaround | ⚠️ Failed in one SRV test | ✅ Separate UDP tunnels |
| [Playit Premium](https://playit.gg/support/playit-premium/) | From $3/month | ✅ Assigned hostname and port | ✅ [External domains](https://playit.gg/support/external-domain-cloudflare/) | ⚠️ Provider example timed out | ✅ Combined TCP+UDP |
| [Localtonet](https://localtonet.com/pricing) | $0 (30 min) or about $2/month per active tunnel, billed hourly | ⚠️ Reserved with balance | ✅ [Domain managed in Localtonet](https://localtonet.com/documents/custom-domains) | ⚠️ [Raw TCP](https://localtonet.com/documents/using-localtonet-with-minecraft) | ✅ Combined TCP+UDP |
| [Pinggy](https://pinggy.io/#pricing) | $0 (60 min) or $3/month | ⚠️ Reserved on Pro | ⚠️ [Pro with assigned port](https://pinggy.io/docs/http_tunnels/multi_port_forwarding/) | ⚠️ [Raw TCP](https://pinggy.io/quickstart/minecraft/) | ✅ Via Pinggy CLI |
| [LocalXpose Pro](https://localxpose.io/pricing) | $8/month, billed yearly | ✅ Reserved port | ⚠️ [CNAME with reserved port](https://localxpose.io/docs/tutorials/access-localhost-keepass) | ⚠️ [Raw TCP](https://localxpose.io/docs/tunnels/tcp) | ✅ Supported |
| [TCPShield](https://tcpshield.com/plans) | From $0 for Java | ✅ Protected hostname | ✅ [CNAME](https://docs.tcpshield.com/panel/dns-setup) | ✅ [Java list ping](https://docs.tcpshield.com/troubleshooting/invalid-hostname) | ⚠️ [Premium Bedrock](https://docs.tcpshield.com/premium-features/geyser) or [Sentry Tunnel](https://docs.tcpshield.com/vxlan/sentry-tunnel-general-setup) |
| [Cloudflare Spectrum](https://developers.cloudflare.com/spectrum/protocols-per-plan/) | [Pro $25/month](https://www.cloudflare.com/plans/pro/) on monthly billing, plus usage | ✅ Fixed hostname and port | ✅ [Cloudflare zone](https://developers.cloudflare.com/spectrum/get-started/) | ✅ Minecraft Java app | ⚠️ Spectrum Enterprise |
| Tovela | From $0 | ✅ Fixed hostname, ephemeral IP | ✅ Cloudflare DNS | ✅ Supported | ✅ Multiple TCP and UDP ports |

ngrok documents [raw TCP forwarding for Minecraft and CNAME aliases](https://ngrok.com/docs/gateway/endpoints/tcp). Native HTTP/TLS domains start at $20/month. [ngrok pricing](https://ngrok.com/pricing)

The Playit checks covered one free Java 1.21.11 tunnel and the [Premium example](https://playit.gg/support/external-domain-cloudflare/). Ping results marked **Raw TCP** for Localtonet, Pinggy, and LocalXpose are inferred from their tunnel documentation.

TCPShield's Java proxy needs a [publicly reachable backend](https://docs.tcpshield.com/miscellaneous/protect-a-home-hosted-server). Spectrum Pro supports one Minecraft Java app and one single-port TCP app with a Worker origin in beta. Generic TCP with an IP or DNS origin and UDP require [Spectrum Enterprise](https://developers.cloudflare.com/spectrum/protocols-per-plan/). [Spectrum billing](https://developers.cloudflare.com/billing/understand/usage-based-billing/)

### Monthly relay cost

![Estimated monthly costs for Iowa and Tokyo compared with Playit Premium](docs/monthly-relay-cost.svg)

At Playit Premium’s $3/month starting price, the example break-even points are **215 h** in Iowa (`us-central1`) and **182 h** in Tokyo (`asia-northeast1`). Costs vary by region. [Playit pricing](https://playit.gg/)

Estimates cover an `e2-micro`, a 10 GiB standard disk, and an IPv4 address while allocated. They exclude transfer, taxes, domain fees, and credits. Eligible `e2-micro` usage in some US regions qualifies for the [Google Cloud Free Tier](https://docs.cloud.google.com/free/docs/free-cloud-features#compute). [Compute pricing](https://cloud.google.com/products/compute/pricing/general-purpose), [disk pricing](https://cloud.google.com/compute/disks-image-pricing), and [IPv4 pricing](https://cloud.google.com/vpc/network-pricing#ipaddress).
