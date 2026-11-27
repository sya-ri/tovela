<h1 align="center">
  <img src="docs/brand/logo.svg" alt="Tovela" width="360">
</h1>

Share Minecraft Java and other local game servers on your own domain. Tovela starts temporary cloud relays for TCP and UDP.

## Requirements

- **Local server** running on the same Windows, macOS, or Linux computer as Tovela.
- **Google Cloud project** with billing, Compute Engine, and a VPC firewall that allows the relay ports.
- **Domain on Cloudflare** with permission to edit its DNS records.

[Cloud requirements](docs/development.md#cloud-requirements) and [design](docs/design.md)

## Service comparison

✅ Supported, 🌐 automatic routing, ⚠️ limited, ❌ unavailable. Checked November 27, 2026.

| Service | Price | Relay routing | Stable endpoint | Custom hostname | UDP |
| --- | --- | --- | --- | --- | --- |
| [ngrok Free](https://ngrok.com/pricing) | $0 (card required) | ⚠️ [Agent location](https://ngrok.com/docs/universal-gateway/global-load-balancer) | ❌ Random address | ⚠️ DNS alias | ❌ |
| [ngrok Hobbyist](https://ngrok.com/docs/gateway/domains/tcp-addresses) | $10/mo | ✅ [8 locations](https://ngrok.com/docs/gateway/points-of-presence) | ✅ Reserved address | ⚠️ [CNAME + port](https://ngrok.com/docs/gateway/domains/custom-domains#using-custom-domains-with-tcp-endpoints) or SRV | ❌ |
| [Playit Free](https://playit.gg/) | $0 | 🌐 [Global Anycast](https://playit.gg/support/playit-premium/) | ✅ Per tunnel | ⚠️ Java SRV | ✅ Separate tunnel |
| [Playit Premium](https://playit.gg/support/playit-premium/) | From $3/mo | ✅ [21 regional networks](https://ping.playit.gg/) | ✅ Per tunnel | ✅ [External domain](https://playit.gg/support/external-domain-cloudflare/) | ✅ Combined TCP+UDP |
| [Localtonet Free](https://localtonet.com/pricing) | $0 (30 min per start, 1 GB per 30 days) | ✅ [20 listed locations](https://localtonet.com/#server-map) | ❌ Port can change | ❌ | ✅ Combined TCP+UDP |
| [Localtonet Pay-as-you-go](https://localtonet.com/documents/plans-and-balance) | ~$2 per tunnel at 720 h, billed hourly | ✅ [20 listed locations](https://localtonet.com/#server-map) | ✅ Reserved port | ✅ [Localtonet DNS](https://localtonet.com/documents/custom-domains) | ✅ Combined TCP+UDP |
| [Pinggy](https://pinggy.io/#pricing) | $0 (60 min) or $3/mo | ✅ [3 documented regions](https://forum.pinggy.io/t/pinggy-latency-may-drift-after-tunnel-restart/112/2) | ⚠️ Pro | ⚠️ [Pro + port](https://pinggy.io/docs/http_tunnels/multi_port_forwarding/) | ✅ Pinggy CLI |
| [LocalXpose Pro](https://localxpose.io/pricing) | $8/mo, billed yearly | ✅ [3 regions](https://localxpose.io/docs#regions) | ✅ Reserved port | ⚠️ [CNAME + port](https://localxpose.io/docs/tutorials/access-localhost-keepass) | ✅ |
| [TCPShield](https://tcpshield.com/plans) | From $0 for Java | 🌐 [Anycast by default](https://docs.tcpshield.com/faq) | ✅ Protected hostname | ✅ [CNAME](https://docs.tcpshield.com/panel/dns-setup) | ⚠️ [Premium Bedrock](https://docs.tcpshield.com/premium-features/geyser) or [Sentry](https://docs.tcpshield.com/vxlan/sentry-tunnel-general-setup) |
| [Cloudflare Spectrum](https://developers.cloudflare.com/spectrum/protocols-per-plan/) | [Pro $25/mo](https://www.cloudflare.com/plans/pro/) + transfer | 🌐 [Global Anycast](https://developers.cloudflare.com/spectrum/reference/configuration-options/) | ✅ Fixed hostname and port | ✅ Cloudflare DNS | ⚠️ Spectrum Enterprise |
| Tovela | From $0 | ✅ [Up to 42 regions](https://docs.cloud.google.com/compute/docs/regions-zones#available) | ✅ Fixed hostname | ✅ Cloudflare DNS | ✅ Multiple ports |

Anycast can route players through nearby edges automatically. Choosing a region keeps the relay in a specific location. [Anycast routing](https://developers.cloudflare.com/cloudflare-wan/glossary/)

TCPShield’s Java proxy [needs a reachable backend](https://docs.tcpshield.com/miscellaneous/protect-a-home-hosted-server).

### Monthly relay cost

![Monthly costs for selected services and hourly estimates for Tovela in Iowa](docs/monthly-relay-cost.svg)

The Tovela estimate uses an e2-micro in Iowa and includes the VM, disk, and IPv4. Estimates exclude transfer, taxes, and credits. [Compute](https://cloud.google.com/products/compute/pricing/general-purpose), [disk](https://cloud.google.com/compute/disks-image-pricing), [IPv4](https://cloud.google.com/vpc/network-pricing#ipaddress), [Free Tier](https://docs.cloud.google.com/free/docs/free-cloud-features#compute)

## Default game server ports

Minecraft Java is the default. For other ports, match your server’s TCP and UDP settings.

| Game | Default ports | Server reference |
| --- | --- | --- |
| Minecraft Java Edition | TCP 25565 | [Minecraft Help](https://help.minecraft.net/hc/en-us/articles/360058525452) |
| Minecraft Bedrock Edition (RakNet) | UDP 19132 | [GeyserMC](https://geysermc.org/wiki/geyser/geyser-command-line-arguments-and-system-properties/) |
| Minecraft Bedrock Edition (NetherNet) | TCP 19132 for signaling, UDP as configured | [Minecraft Help](https://help.minecraft.net/hc/en-us/articles/49914677714957) |
| 7 Days to Die | TCP 26900, UDP 26900–26903 | [7 Days to Die Help Center](https://support.7daystodie.com/hc/en-us/articles/48451426400916-Opening-Ports-in-Firewall) |
| BeamMP | TCP and UDP 30814 | [BeamMP Docs](https://docs.beammp.com/server/port-forwarding/) |
| Factorio | UDP 34197 | [Factorio Wiki](https://wiki.factorio.com/Multiplayer) |
| Hytale | UDP 5520 | [Hytale Server Manual](https://support.hytale.com/hc/en-us/articles/45326769420827-Hytale-Server-Manual) |
| OpenTTD (direct IP) | TCP and UDP 3979 | [OpenTTD Manual](https://wiki.openttd.org/en/Manual/FAQ%20multiplayer) |
| Palworld | UDP 8211 | [Palworld Server Guide](https://docs.palworldgame.com/getting-started/requirements/) |
| Project Zomboid (Steam) | UDP 16261 and 16262 | [The Indie Stone](https://theindiestone.com/forums/topic/60581-4177-released/) |
| Rust | UDP 28015 for direct connections | [Facepunch Wiki](https://wiki.facepunch.com/rust/creating-a-server) |
| Satisfactory | TCP and UDP 7777, TCP 8888 | [Satisfactory Wiki](https://satisfactory.wiki.gg/wiki/Dedicated_servers) |
| Space Engineers | UDP 27016 | [Space Engineers](https://www.spaceengineersgame.com/dedicated-servers/#router-and-firewall-settings) |
| Terraria | TCP 7777 | [Terraria Wiki](https://terraria.wiki.gg/wiki/Server#Before_You_Host) |
| Unturned | UDP 27015 and 27016 | [Unturned Docs](https://docs.smartlydressedgames.com/en/latest/servers/port-forwarding.html) |
| Valheim (Steam backend) | UDP 2456 and 2457 | [Valheim Server Guide](https://www.valheimgame.com/support/a-guide-to-dedicated-servers/) |
| Vintage Story | TCP 42420 | [Vintage Story Wiki](https://wiki.vintagestory.at/Setting_up_a_Multiplayer_Server/en) |
