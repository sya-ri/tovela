package relay

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/sya-ri/tovela/internal/locale"
)

type PortProtocol string

const (
	MinecraftJava PortProtocol = "Minecraft Java"
	TCP           PortProtocol = "TCP"
	UDP           PortProtocol = "UDP"
)

type PortMapping struct {
	Protocol   PortProtocol
	LocalPort  int
	PublicPort int
}

func (p PortMapping) Transport() string {
	if p.Protocol == UDP {
		return "udp"
	}
	return "tcp"
}

func (p PortMapping) proxyName() string {
	if p.Protocol == MinecraftJava && p.PublicPort == DefaultTCPPort {
		return "minecraft-java"
	}
	return fmt.Sprintf("%s-%d", p.Transport(), p.PublicPort)
}

func (p PortMapping) probeTCP(ctx context.Context, ip, host string, port int) error {
	address := net.JoinHostPort(ip, strconv.Itoa(port))
	if p.Protocol == MinecraftJava {
		return pingMinecraft(ctx, address, host, port)
	}
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	return conn.Close()
}

func validatePortMappings(ports []PortMapping) error {
	public := make(map[string]bool, len(ports))
	for _, port := range ports {
		switch port.Protocol {
		case MinecraftJava, TCP, UDP:
		default:
			return errors.New(locale.T("Choose a supported protocol for each port."))
		}
		if port.LocalPort < 1 || port.LocalPort > 65535 || port.PublicPort < 1 || port.PublicPort > 65535 {
			return errors.New(locale.T("Enter local and public ports from 1 to 65535."))
		}
		if port.Protocol != UDP && (port.PublicPort == 22 || port.PublicPort == 7000) {
			return fmt.Errorf(locale.T("Public TCP port %d is reserved for the relay."), port.PublicPort)
		}
		key := fmt.Sprintf("%s-%d", port.Transport(), port.PublicPort)
		if public[key] {
			return fmt.Errorf(locale.T("Public %s port %d is used more than once."), port.Transport(), port.PublicPort)
		}
		public[key] = true
	}
	return nil
}

// legacyPorts preserves profiles and running sessions saved with a single TCP and optional UDP port.
type legacyPorts struct {
	TCPPort int `json:",omitempty"`
	UDPPort int `json:",omitempty"`
}

func (p legacyPorts) mappings() []PortMapping {
	tcp := p.TCPPort
	if tcp == 0 {
		tcp = DefaultTCPPort
	}
	ports := []PortMapping{{MinecraftJava, tcp, DefaultTCPPort}}
	if p.UDPPort != 0 {
		ports = append(ports, PortMapping{UDP, p.UDPPort, p.UDPPort})
	}
	return ports
}
