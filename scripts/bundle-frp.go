package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const version = "0.72.0"

var checksums = map[string]string{
	"darwin_amd64.tar.gz": "60439dc6bf794c4182a1a4cac147d642f839d55201607208979eb23aecd09be9",
	"darwin_arm64.tar.gz": "6e8576c5b6b9914a86850f6ff4fb81c5a09e706f91f70f64d5f398558bdc9db8",
	"linux_amd64.tar.gz":  "daf719b32ca2778770bb92955e231a07414d754378b9af328aa2642862479804",
	"linux_arm64.tar.gz":  "02180b857480302fd934e9ddbd50fe2f73229f99062f88756abc6de9df5e95cf",
	"windows_amd64.zip":   "5197c1b69564089cdc229fc1260a0e25db1e722b1b7db45fe1542c3c43edf9ae",
	"windows_arm64.zip":   "e4b83b66c4045971dd9a47cdef16dd02af471aaa6667dd303e63b1a669162927",
}

func main() {
	if err := bundle(); err != nil {
		log.Fatal(err)
	}
}

func bundle() error {
	extension, binary := ".tar.gz", "frpc"
	if runtime.GOOS == "windows" {
		extension, binary = ".zip", "frpc.exe"
	}
	target := runtime.GOOS + "_" + runtime.GOARCH
	folder := "frp_" + version + "_" + target
	checksum, supported := checksums[target+extension]
	if !supported {
		return fmt.Errorf("no frp build for %s", target)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	archive := folder + extension
	reply, err := client.Get("https://github.com/fatedier/frp/releases/download/v" + version + "/" + archive)
	if err != nil {
		return err
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		return fmt.Errorf("frp download failed: %s", reply.Status)
	}
	data, err := io.ReadAll(io.LimitReader(reply.Body, 64<<20))
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != checksum {
		return fmt.Errorf("checksum mismatch for %s", archive)
	}

	temp, err := os.MkdirTemp("", "tovela-frp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	path := filepath.Join(temp, archive)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	output, err := exec.Command("tar", "-xf", path, "-C", temp, folder+"/"+binary, folder+"/LICENSE").CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract frp: %w: %s", err, output)
	}
	for member, dest := range map[string]string{
		binary:    filepath.Join("internal", "relay", "bin", binary),
		"LICENSE": filepath.Join("build", "frp-LICENSE"),
	} {
		content, err := os.ReadFile(filepath.Join(temp, folder, member))
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, content, 0755); err != nil {
			return err
		}
	}
	fmt.Printf("Bundled %s %s for %s\n", binary, version, target)
	return nil
}
