package sourcebook_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestShellInstallerDownloadsVerifiesAndInstallsRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer is not used on Windows")
	}

	fixture := newInstallerFixture(t, false)
	binDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command("sh", "install.sh", "--version", fixture.version, "--bin-dir", binDir)
	command.Env = append(os.Environ(),
		"SOURCEBOOK_RELEASE_BASE_URL=file://"+fixture.root,
		"SOURCEBOOK_OS="+fixture.targetOS,
		"SOURCEBOOK_ARCH="+fixture.targetArch,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}

	installed := filepath.Join(binDir, "sourcebook")
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("installed binary: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("installed binary mode = %v, want executable", info.Mode())
	}
	versionOutput, err := exec.Command(installed, "version").CombinedOutput()
	if err != nil {
		t.Fatalf("installed sourcebook failed: %v\n%s", err, versionOutput)
	}
	if got, want := strings.TrimSpace(string(versionOutput)), "sourcebook v"+fixture.version; got != want {
		t.Fatalf("installed version = %q, want %q", got, want)
	}
	if !strings.Contains(string(output), installed) {
		t.Fatalf("installer output does not identify %q:\n%s", installed, output)
	}
}

func TestShellInstallerUsesDocumentedDefaultDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer is not used on Windows")
	}

	fixture := newInstallerFixture(t, false)
	homeDir := t.TempDir()
	command := exec.Command("sh", "install.sh", "--version", fixture.version)
	command.Env = append(os.Environ(),
		"HOME="+homeDir,
		"SOURCEBOOK_INSTALL_DIR=",
		"SOURCEBOOK_RELEASE_BASE_URL=file://"+fixture.root,
		"SOURCEBOOK_OS="+fixture.targetOS,
		"SOURCEBOOK_ARCH="+fixture.targetArch,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	installed := filepath.Join(homeDir, ".local", "bin", "sourcebook")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("default installed binary stat error = %v", err)
	}
}

func TestShellInstallerDefaultsToLatestSourcebookRelease(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer is not used on Windows")
	}

	fixture := newInstallerFixture(t, false)
	binDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command("sh", "install.sh", "--bin-dir", binDir)
	command.Env = append(os.Environ(),
		"SOURCEBOOK_VERSION=",
		"SOURCEBOOK_RELEASE_BASE_URL=file://"+fixture.root,
		"SOURCEBOOK_RELEASES_API_URL=file://"+filepath.Join(fixture.root, "releases.json"),
		"SOURCEBOOK_OS="+fixture.targetOS,
		"SOURCEBOOK_ARCH="+fixture.targetArch,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "v"+fixture.version) {
		t.Fatalf("installer output does not contain resolved version:\n%s", output)
	}
}

func TestShellInstallerRejectsInvalidChecksum(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX installer is not used on Windows")
	}

	fixture := newInstallerFixture(t, true)
	binDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command("sh", "install.sh", "--version", fixture.version, "--bin-dir", binDir)
	command.Env = append(os.Environ(),
		"SOURCEBOOK_RELEASE_BASE_URL=file://"+fixture.root,
		"SOURCEBOOK_OS="+fixture.targetOS,
		"SOURCEBOOK_ARCH="+fixture.targetArch,
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("install.sh succeeded with invalid checksum:\n%s", output)
	}
	if !strings.Contains(strings.ToLower(string(output)), "checksum") {
		t.Fatalf("installer error does not mention checksum:\n%s", output)
	}
	if _, err := os.Stat(filepath.Join(binDir, "sourcebook")); !os.IsNotExist(err) {
		t.Fatalf("binary exists after rejected download: %v", err)
	}
}

func TestPowerShellInstallerContainsRequiredSafetyChecks(t *testing.T) {
	contents, err := os.ReadFile("install.ps1")
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		`$env:OS`,
		`Windows_NT`,
		`$env:PROCESSOR_ARCHITEW6432`,
		`$env:PROCESSOR_ARCHITECTURE`,
		"Get-FileHash",
		"checksums.txt",
		"Expand-Archive",
		"SOURCEBOOK_INSTALL_DIR",
		"SOURCEBOOK_RELEASES_API_URL",
		"ConvertFrom-Json",
		`Programs\OpenAI\Codex\bin`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("install.ps1 does not contain %q", required)
		}
	}
	if strings.Contains(script, `$DefaultVersion = "0.1.0"`) {
		t.Error("install.ps1 still defaults to the initial release")
	}
	if strings.Contains(script, `Programs\Sourcebook`) {
		t.Error("install.ps1 still uses the old nonstandard Windows install directory")
	}
	if strings.Contains(script, "RuntimeInformation") {
		t.Error("install.ps1 relies on RuntimeInformation, which is unavailable in some Windows PowerShell environments")
	}
	wow64 := strings.Index(script, `$env:PROCESSOR_ARCHITEW6432`)
	native := strings.Index(script, `$env:PROCESSOR_ARCHITECTURE`)
	if wow64 < 0 || native < 0 || wow64 > native {
		t.Error("install.ps1 must prefer PROCESSOR_ARCHITEW6432 before PROCESSOR_ARCHITECTURE")
	}
}

func TestPowerShellInstallerDownloadsVerifiesAndInstallsRelease(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows PowerShell installer requires Windows")
	}

	targetArch := runtime.GOARCH
	if targetArch != "amd64" && targetArch != "arm64" {
		t.Skipf("unsupported Windows test architecture %s", targetArch)
	}
	fixtureRoot := t.TempDir()
	version := "9.8.7"
	releaseDir := filepath.Join(fixtureRoot, "sourcebook", "v"+version)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetName := fmt.Sprintf("sourcebook_%s_windows_%s.zip", version, targetArch)
	assetPath := filepath.Join(releaseDir, assetName)
	writeZip(t, assetPath, map[string][]byte{
		"sourcebook.exe": []byte("sourcebook fixture"),
	})
	assetContents, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	checksums := fmt.Sprintf("%x  %s\n", sha256.Sum256(assetContents), assetName)
	if err := os.WriteFile(filepath.Join(releaseDir, "checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(t.TempDir(), "bin")
	command := exec.Command(
		"powershell.exe",
		"-NoLogo",
		"-NoProfile",
		"-ExecutionPolicy", "Bypass",
		"-File", "install.ps1",
		"-Version", version,
		"-BinDir", binDir,
	)
	releaseBaseURL := "file:///" + strings.TrimPrefix(filepath.ToSlash(fixtureRoot), "/")
	command.Env = append(os.Environ(),
		"SOURCEBOOK_RELEASE_BASE_URL="+releaseBaseURL,
		"SOURCEBOOK_OS=",
		"SOURCEBOOK_ARCH=",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("install.ps1 failed: %v\n%s", err, output)
	}
	installed := filepath.Join(binDir, "sourcebook.exe")
	if _, err := os.Stat(installed); err != nil {
		t.Fatalf("installed binary stat error = %v", err)
	}
	if !strings.Contains(string(output), "Sourcebook v"+version+" installed") {
		t.Fatalf("installer output does not report success:\n%s", output)
	}
}

func TestREADMEDocumentsInstallers(t *testing.T) {
	contents, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(contents)
	for _, command := range []string{
		"curl -fsSL https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.sh | sh",
		"irm https://raw.githubusercontent.com/Yacobolo/toolbelt/main/sourcebook/install.ps1 | iex",
		"sourcebook upgrade",
	} {
		if !strings.Contains(readme, command) {
			t.Errorf("README.md does not contain %q", command)
		}
	}
	for _, documentedPath := range []string{
		"CODEX_HOME",
		`%LOCALAPPDATA%\Programs\OpenAI\Codex\bin`,
		"~/.local/bin",
	} {
		if !strings.Contains(readme, documentedPath) {
			t.Errorf("README.md does not contain %q", documentedPath)
		}
	}
}

func TestInstallersExplainShellCommandCaching(t *testing.T) {
	for _, filename := range []string{"install.sh", "install.ps1"} {
		contents, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}
		lower := strings.ToLower(string(contents))
		if !strings.Contains(lower, "older version") {
			t.Errorf("%s does not explain what to do when the shell resolves an older version", filename)
		}
	}
}

type installerFixture struct {
	root       string
	version    string
	targetOS   string
	targetArch string
}

func newInstallerFixture(t *testing.T, invalidChecksum bool) installerFixture {
	t.Helper()
	fixture := installerFixture{
		root:       t.TempDir(),
		version:    "9.8.7",
		targetOS:   "darwin",
		targetArch: "arm64",
	}
	releaseDir := filepath.Join(fixture.root, "sourcebook", "v"+fixture.version)
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	assetName := fmt.Sprintf("sourcebook_%s_%s_%s.tar.gz", fixture.version, fixture.targetOS, fixture.targetArch)
	assetPath := filepath.Join(releaseDir, assetName)
	binary := []byte("#!/bin/sh\nprintf 'sourcebook v9.8.7\\n'\n")
	writeTarGz(t, assetPath, map[string]archiveFile{
		"sourcebook": {contents: binary, mode: 0o755},
		"README.md":  {contents: []byte("fixture"), mode: 0o644},
	})

	assetContents, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(assetContents))
	if invalidChecksum {
		digest = strings.Repeat("0", 64)
	}
	checksums := fmt.Sprintf("%s  %s\n", digest, assetName)
	if err := os.WriteFile(filepath.Join(releaseDir, "checksums.txt"), []byte(checksums), 0o644); err != nil {
		t.Fatal(err)
	}
	releases := fmt.Sprintf(`[
  {"tag_name":"apigen/v99.0.0","draft":false,"prerelease":false},
  {"tag_name":"sourcebook/v1.0.0","draft":false,"prerelease":false},
  {"tag_name":"sourcebook/v%s","draft":false,"prerelease":false}
]`, fixture.version)
	if err := os.WriteFile(filepath.Join(fixture.root, "releases.json"), []byte(releases), 0o644); err != nil {
		t.Fatal(err)
	}
	return fixture
}

type archiveFile struct {
	contents []byte
	mode     int64
}

func writeTarGz(t *testing.T, filename string, files map[string]archiveFile) {
	t.Helper()
	output, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, file := range files {
		header := &tar.Header{Name: name, Mode: file.mode, Size: int64(len(file.contents))}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(tarWriter, bytes.NewReader(file.contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, filename string, files map[string][]byte) {
	t.Helper()
	output, err := os.Create(filename)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(output)
	for name, contents := range files {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(contents); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}
