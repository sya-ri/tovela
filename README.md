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

Checked November 27, 2026.

| Service | Price | Relay routing | Relay CPU / RAM | Stable endpoint | Custom hostname | Multiple custom hostnames | UDP | Multiple public ports |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| [ngrok Free](https://ngrok.com/pricing) | $0 (card required) | ⚠️ [Agent location](https://ngrok.com/docs/universal-gateway/global-load-balancer) | Managed | ❌ Random address | ⚠️ DNS alias | ⚠️ [TCP aliases](https://ngrok.com/docs/gateway/domains/custom-domains#using-custom-domains-with-tcp-endpoints), random endpoints | ❌ | ✅ [Up to 3 endpoints](https://ngrok.com/docs/pricing-limits) within usage limits |
| [ngrok Hobbyist](https://ngrok.com/docs/gateway/domains/tcp-addresses) | $10/mo | ✅ [8 locations](https://ngrok.com/docs/gateway/points-of-presence) | Managed | ✅ Reserved address | ⚠️ [CNAME + port](https://ngrok.com/docs/gateway/domains/custom-domains#using-custom-domains-with-tcp-endpoints) or SRV | ⚠️ TCP aliases, 1 fixed TCP address included | ❌ | ✅ [Up to 3 endpoints](https://ngrok.com/docs/pricing-limits) within credit, 1 fixed TCP address included |
| [Playit Free](https://playit.gg/) | $0 | 🌐 [Global Anycast](https://playit.gg/support/playit-premium/) | Managed | ✅ Per tunnel | ⚠️ Java SRV | ⚠️ Java SRV aliases | ✅ Separate tunnel | ✅ [4 ports included](https://playit.gg/support/playit-premium/#more) for supported tunnel types |
| [Playit Premium](https://playit.gg/support/playit-premium/) | From $3/mo | ✅ [21 regional networks](https://ping.playit.gg/) | Managed | ✅ Per tunnel | ✅ [External domain](https://playit.gg/support/external-domain-cloudflare/) | ✅ External names assignable per tunnel | ✅ Combined TCP+UDP | ✅ [16 ports included](https://playit.gg/support/playit-premium/#more) |
| [Localtonet Free](https://localtonet.com/pricing) | $0 (30 min per start, 1 GB per 30 days) | ✅ [20 listed locations](https://localtonet.com/#server-map) | Managed | ❌ Port can change | ❌ | ❌ | ✅ Combined TCP+UDP | ❌ 1 tunnel, [1 public port](https://localtonet.com/documents/udp-tcp) |
| [Localtonet Pay-as-you-go](https://localtonet.com/documents/plans-and-balance) | ~$2 per tunnel at 720 h, billed hourly | ✅ [20 listed locations](https://localtonet.com/#server-map) | Managed | ✅ Reserved port | ✅ [Localtonet DNS](https://localtonet.com/documents/custom-domains) | ⚠️ 1 billed tunnel per hostname | ✅ Combined TCP+UDP | ⚠️ Multiple billed tunnels, 1 public port each |
| [Pinggy](https://pinggy.io/#pricing) | $0 (60 min) or $3/mo | ✅ [3 documented regions](https://forum.pinggy.io/t/pinggy-latency-may-drift-after-tunnel-restart/112/2) | Managed | ⚠️ Pro | ⚠️ [Pro + port](https://pinggy.io/docs/http_tunnels/multi_port_forwarding/) | ⚠️ [Pro wildcard routing](https://pinggy.io/docs/http_tunnels/multi_port_forwarding/), raw TCP routes by port | ✅ Pinggy CLI | ✅ [Multiple forwardings](https://pinggy.io/docs/http_tunnels/multi_port_forwarding/), Pro priced per seat |
| [LocalXpose Pro](https://localxpose.io/pricing) | $8/mo, billed yearly | ✅ [3 regions](https://localxpose.io/docs#regions) | Managed | ✅ Reserved port | ⚠️ [CNAME + port](https://localxpose.io/docs/tutorials/access-localhost-keepass) | ⚠️ TCP aliases, 10 reservations per slot | ✅ | ✅ 10 active tunnels included per slot |
| [Cloudflare Spectrum](https://developers.cloudflare.com/spectrum/protocols-per-plan/) | [Pro $25/mo](https://www.cloudflare.com/plans/pro/) + transfer | 🌐 [Global Anycast](https://developers.cloudflare.com/spectrum/reference/configuration-options/) | Managed | ✅ Fixed hostname and port | ✅ Cloudflare DNS | ⚠️ [Aliases share one origin](https://developers.cloudflare.com/spectrum/reference/limitations/), 1 Minecraft app on Pro | ⚠️ Spectrum Enterprise | ⚠️ [Pro app limits](https://developers.cloudflare.com/spectrum/protocols-per-plan/), Spectrum Enterprise for more protocols |
| Tovela | From $0 | ✅ [Up to 42 regions](https://docs.cloud.google.com/compute/docs/regions-zones#available) | 0.25–2 sustained vCPU, 1–8 GiB | ✅ Fixed hostname | ✅ Cloudflare DNS | ⚠️ [1 VM per active hostname](docs/design.md) | ✅ | ✅ Multiple per VM, no per-port fee |

Anycast can route players through nearby edges automatically. Choosing a region keeps the relay in a specific location. [Anycast routing](https://developers.cloudflare.com/cloudflare-wan/glossary/)

### Monthly relay cost

![Monthly costs by hours online for selected services and four Tovela machine types in Iowa](docs/monthly-relay-cost.svg)

The chart uses one Localtonet tunnel (one public port). Tovela estimates use Iowa rates with a 10 GiB standard disk and public IPv4. Estimates exclude transfer, taxes, and credits. [Compute](https://cloud.google.com/products/compute/pricing/general-purpose), [disk](https://cloud.google.com/compute/disks-image-pricing), [IPv4](https://cloud.google.com/vpc/network-pricing#ipaddress), [Free Tier](https://docs.cloud.google.com/free/docs/free-cloud-features#compute)


The three shared-core types can briefly burst to two vCPUs. [E2 machine types](https://docs.cloud.google.com/compute/docs/general-purpose-machines#sharedcore)

> [!NOTE]
> For most setups, we recommend Localtonet. Tovela is a good fit if you want your own relay or need to publish several ports at once.

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
