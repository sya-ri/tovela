package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/sya-ri/tovela/internal/frprelease"
)

const maxArchiveSize = 64 << 20

var checksums = map[string]string{
	"darwin_amd64.tar.gz": "60439dc6bf794c4182a1a4cac147d642f839d55201607208979eb23aecd09be9",
	"darwin_arm64.tar.gz": "6e8576c5b6b9914a86850f6ff4fb81c5a09e706f91f70f64d5f398558bdc9db8",
	"linux_amd64.tar.gz":  frprelease.LinuxAMD64SHA256,
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
	folder := "frp_" + frprelease.Version + "_" + target
	checksum, supported := checksums[target+extension]
	if !supported {
		return fmt.Errorf("no frp build for %s", target)
	}
	client := &http.Client{Timeout: 90 * time.Second}
	archive := folder + extension
	reply, err := client.Get("https://github.com/fatedier/frp/releases/download/v" + frprelease.Version + "/" + archive)
	if err != nil {
		return err
	}
	defer reply.Body.Close()
	if reply.StatusCode != http.StatusOK {
		return fmt.Errorf("frp download failed: %s", reply.Status)
	}

	temp, err := os.MkdirTemp("", "tovela-frp-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	path := filepath.Join(temp, archive)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(reply.Body, maxArchiveSize+1))
	if err := errors.Join(copyErr, file.Close()); err != nil {
		return err
	}
	if size > maxArchiveSize {
		return fmt.Errorf("frp archive exceeds %d bytes: %s", maxArchiveSize, archive)
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != checksum {
		return fmt.Errorf("checksum mismatch for %s", archive)
	}
	output, err := exec.Command("tar", "-xf", path, "-C", temp, folder+"/"+binary, folder+"/LICENSE").CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract frp: %w: %s", err, output)
	}
	for member, dest := range map[string]string{
		binary:    filepath.Join("internal", "relay", "bin", binary),
		"LICENSE": filepath.Join("build", "frp-LICENSE"),
	} {
		if err := install(filepath.Join(temp, folder, member), dest); err != nil {
			return err
		}
	}
	fmt.Printf("Bundled %s %s for %s\n", binary, frprelease.Version, target)
	return nil
}

func install(source, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	// Stage the copy alongside the destination so the final rename stays on one volume.
	output, err := os.CreateTemp(filepath.Dir(dest), ".frp-*")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	_, copyErr := io.Copy(output, input)
	if err := errors.Join(copyErr, output.Chmod(0755), output.Close()); err != nil {
		return err
	}
	return os.Rename(output.Name(), dest)
}
