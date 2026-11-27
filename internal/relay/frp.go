package relay

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/sya-ri/tovela/internal/frprelease"
	"github.com/sya-ri/tovela/internal/locale"
)

// ErrFrpcBlocked indicates that Windows denied access to the bundled client.
var ErrFrpcBlocked = errors.New("Windows blocked frpc")

//go:embed all:bin
var frpFiles embed.FS

type identity struct {
	RequestID       string
	Nonce           string
	Token           string
	Certificate     []byte
	PrivateKey      []byte
	Deadline        time.Time
	InstanceID      string
	DNSZoneID       string
	PublicIP        string
	PortMappingHash string `json:",omitempty"`
	legacyPorts
}

func newIdentity(config Config) (identity, error) {
	session := identity{
		RequestID: requestID(), Nonce: randomHex(16), Token: randomHex(32),
		Deadline:        time.Now().UTC().Add(time.Duration(config.Hours) * time.Hour).Truncate(time.Second),
		PortMappingHash: portMappingHash(config.Ports),
	}
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return identity{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return identity{}, err
	}
	certificate := &x509.Certificate{
		// The SAN carries the hostname. Repeating it in the subject and issuer can exceed the Windows credential limit.
		SerialNumber: serial,
		DNSNames:     []string{config.Domain}, NotBefore: time.Now().Add(-5 * time.Minute), NotAfter: time.Now().Add(121 * 24 * time.Hour),
		BasicConstraintsValid: true, IsCA: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &private.PublicKey, private)
	if err != nil {
		return identity{}, err
	}
	key, err := x509.MarshalECPrivateKey(private)
	if err != nil {
		return identity{}, err
	}
	session.Certificate = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	session.PrivateKey = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: key})
	return session, nil
}

// portMappingHash identifies the saved mappings without growing the credential with each port.
func portMappingHash(ports []PortMapping) string {
	hash := sha256.New()
	for _, port := range ports {
		fmt.Fprintf(hash, "%q %d %d\n", port.Protocol, port.LocalPort, port.PublicPort)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (s identity) matchesPorts(ports []PortMapping) bool {
	if s.PortMappingHash != "" {
		return s.PortMappingHash == portMappingHash(ports)
	}
	return slices.Equal(s.legacyPorts.mappings(), ports)
}

func serverStartup(session identity, mappings []PortMapping) string {
	folder := "frp_" + frprelease.Version + "_linux_amd64"
	download := "https://github.com/fatedier/frp/releases/download/v" + frprelease.Version + "/" + folder + ".tar.gz"
	var allowed []string
	seen := make(map[int]bool)
	for _, port := range mappings {
		if !seen[port.PublicPort] {
			allowed = append(allowed, fmt.Sprintf("{single = %d}", port.PublicPort))
			seen[port.PublicPort] = true
		}
	}
	ports := "[" + strings.Join(allowed, ", ") + "]"
	capabilities := "CapabilityBoundingSet=\n"
	if slices.ContainsFunc(mappings, func(port PortMapping) bool { return port.PublicPort < 1024 }) {
		capabilities = "CapabilityBoundingSet=CAP_NET_BIND_SERVICE\nAmbientCapabilities=CAP_NET_BIND_SERVICE\n"
	}
	return fmt.Sprintf(`#!/bin/sh
set -eu
umask 077

id -u tovela >/dev/null 2>&1 || useradd --system --user-group --no-create-home --shell /usr/sbin/nologin tovela
install -d -m 0750 -o root -g tovela /opt/tovela
cd /opt/tovela

if ! command -v curl >/dev/null; then
  apt-get update && apt-get install -y --no-install-recommends curl ca-certificates
fi
curl --fail --silent --show-error --location --retry 2 --max-time 90 '%s' -o frp.tar.gz
printf '%%s  frp.tar.gz\n' '%s' | sha256sum -c -
tar -xzf frp.tar.gz --strip-components=1 '%s/frps'
rm -f frp.tar.gz

printf '%%s' '%s' | base64 -d > server.crt
printf '%%s' '%s' | base64 -d > server.key
cat > frps.toml <<'FRP_CONFIG'
bindPort = 7000
allowPorts = %s
auth.token = %q
transport.tls.force = true
transport.tls.certFile = "/opt/tovela/server.crt"
transport.tls.keyFile = "/opt/tovela/server.key"
FRP_CONFIG
chown root:tovela frps frps.toml server.crt server.key
chmod 0750 frps
chmod 0640 frps.toml server.crt server.key

cat > /etc/systemd/system/tovela.service <<'FRP_UNIT'
[Unit]
After=network-online.target

[Service]
WorkingDirectory=/opt/tovela
ExecStart=/opt/tovela/frps -c /opt/tovela/frps.toml
User=tovela
Group=tovela
%sRestart=on-failure
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
FRP_UNIT
systemctl daemon-reload
systemctl enable --now tovela.service
`, download, frprelease.LinuxAMD64SHA256, folder, base64.StdEncoding.EncodeToString(session.Certificate),
		base64.StdEncoding.EncodeToString(session.PrivateKey), ports, session.Token, capabilities)
}

func frpcConfig(config Config, session identity, ip, certPath string) string {
	var result strings.Builder
	fmt.Fprintf(&result, "serverAddr = %q\nserverPort = 7000\nloginFailExit = false\nauth.token = %q\n", ip, session.Token)
	fmt.Fprintf(&result, "transport.tls.enable = true\ntransport.tls.trustedCaFile = %q\ntransport.tls.serverName = %q\n", filepath.ToSlash(certPath), config.Domain)
	for _, port := range config.Ports {
		fmt.Fprintf(&result, "\n[[proxies]]\nname = %q\ntype = %q\nlocalIP = \"127.0.0.1\"\nlocalPort = %d\nremotePort = %d\n",
			port.proxyName(), port.Transport(), port.LocalPort, port.PublicPort)
	}
	return result.String()
}

type tunnel struct {
	cmd      *exec.Cmd
	done     chan struct{}
	udpReady chan string
	err      error
}

func prepareFrpc(ctx context.Context) (path string, err error) {
	name := "frpc"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary, err := frpFiles.Open("bin/" + name)
	if err != nil {
		return "", errors.New(locale.T("This build does not contain frpc for your OS."))
	}
	defer binary.Close()
	dir, err := os.MkdirTemp("", "tovela-frpc-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(dir)
		}
	}()

	executable := filepath.Join(dir, name)
	file, err := os.OpenFile(executable, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return "", frpcExecutionError(err)
	}
	_, copyErr := io.Copy(file, binary)
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return "", frpcExecutionError(err)
	}
	// Detect local execution restrictions before provisioning a billable VM.
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	probe := exec.CommandContext(checkCtx, executable, "--version")
	hideConsole(probe)
	if err := probe.Run(); err != nil {
		return "", frpcExecutionError(fmt.Errorf(locale.T("could not run bundled frpc: %w"), err))
	}
	return executable, nil
}

func launchFrpc(executable string, config Config, session identity, ip string) (*tunnel, error) {
	dir := filepath.Dir(executable)
	certPath := filepath.Join(dir, "server.crt")
	configPath := filepath.Join(dir, "frpc.toml")
	for _, file := range []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{certPath, session.Certificate, 0600},
		{configPath, []byte(frpcConfig(config, session, ip, certPath)), 0600},
	} {
		if err := os.WriteFile(file.path, file.data, file.mode); err != nil {
			return nil, err
		}
	}
	cmd := exec.Command(executable, "-c", configPath)
	hideConsole(cmd)
	logs, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = cmd.Stdout
	t := &tunnel{cmd: cmd, done: make(chan struct{}), udpReady: make(chan string, len(config.Ports))}
	if err := cmd.Start(); err != nil {
		return nil, frpcExecutionError(err)
	}
	go func() {
		pendingUDP := make(map[string]bool)
		for _, port := range config.Ports {
			if port.Protocol == UDP {
				pendingUDP[port.proxyName()] = true
			}
		}
		lines := bufio.NewScanner(logs)
		for lines.Scan() {
			line := lines.Text()
			if strings.Contains(line, "start proxy success") {
				for name := range pendingUDP {
					if strings.Contains(line, "["+name+"]") {
						t.udpReady <- name
						delete(pendingUDP, name)
					}
				}
			}
		}
		_, _ = io.Copy(io.Discard, logs)
		t.err = cmd.Wait()
		_ = os.RemoveAll(dir)
		close(t.done)
	}()
	return t, nil
}

func (t *tunnel) stop() {
	if t == nil {
		return
	}
	_ = t.cmd.Process.Kill()
	select {
	case <-t.done:
	case <-time.After(3 * time.Second):
	}
}

func (t *tunnel) waitReady(ctx context.Context, config Config, ip string) error {
	registered := make(map[string]bool)
	for {
		select {
		case <-t.done:
			return fmt.Errorf(locale.T("The relay client exited: %v"), t.err)
		default:
		}
		for len(t.udpReady) > 0 {
			registered[<-t.udpReady] = true
		}

		var pending *PortMapping
		for _, port := range config.Ports {
			if port.Protocol == UDP {
				if !registered[port.proxyName()] {
					pending = &port
					break
				}
				continue
			}
			if port.probeTCP(ctx, ip, config.Domain, port.PublicPort) != nil {
				pending = &port
				break
			}
		}
		if pending == nil {
			return nil
		}
		if err := pause(ctx, 2*time.Second); err != nil {
			return fmt.Errorf(locale.T("Public %s port %d did not become ready. Check the GCE firewall and local service."),
				strings.ToUpper(pending.Transport()), pending.PublicPort)
		}
	}
}
