// assets_integration_test.go exercises asset validation and serving through the actual CLI.
package cli_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"analysis/tests/support/go/testenv"
	"analysis/tests/support/go/testfixture"
)

// TestFrontendAssets verifies directory validation and serves bytes through the public executable.
func TestFrontendAssets(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "analysis")
	build := exec.Command("go", "build", "-o", binary, "./backend/cmd/analysis")
	build.Dir = testenv.Root()
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	assets := t.TempDir()
	if err := os.WriteFile(filepath.Join(assets, "index.html"), []byte("filesystem asset"), 0644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(assets, "not-a-directory")
	if err := os.WriteFile(file, []byte("file"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{filepath.Join(assets, "missing"), file} {
		command := exec.Command(binary, "serve", "--db", "missing.db", "--assets-dir", invalid)
		command.Dir = testenv.Root()
		output, err := command.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 || !strings.Contains(string(output), "frontend asset directory") {
			t.Fatalf("invalid asset path status=%v output=%s", err, output)
		}
	}
	dbPath := filepath.Join(t.TempDir(), "metadata.db")
	db, err := testfixture.Open(dbPath, testenv.Path("config", "database.something"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, binary, "serve", "--db", dbPath, "--addr", "127.0.0.1:0", "--assets-dir", assets)
	command.Dir = testenv.Root()
	stderr, err := command.StderrPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	defer func() { cancel(); _ = command.Wait() }()
	ready := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stderr)
		address := regexp.MustCompile(`addr=([^\s]+)`)
		for scanner.Scan() {
			if strings.Contains(scanner.Text(), "viewer listening") {
				if match := address.FindStringSubmatch(scanner.Text()); match != nil {
					ready <- match[1]
					return
				}
			}
		}
		ready <- ""
	}()
	var address string
	select {
	case address = <-ready:
	case <-time.After(30 * time.Second):
		t.Fatal("CLI viewer did not start")
	}
	if address == "" {
		t.Fatal("CLI viewer exited before listening")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(fmt.Sprintf("http://%s/", address))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	contents, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK || string(contents) != "filesystem asset" {
		t.Fatalf("served assets: status=%d body=%q err=%v", response.StatusCode, contents, err)
	}
}
