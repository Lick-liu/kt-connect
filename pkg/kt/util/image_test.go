package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultKtImagesUseChengduRegistry(t *testing.T) {
	if ImageKtShadow != "registry.cn-chengdu.aliyuncs.com/zodancer/kt-connect-shadow" {
		t.Fatalf("ImageKtShadow = %q", ImageKtShadow)
	}
	if ImageKtRouter != "registry.cn-chengdu.aliyuncs.com/zodancer/kt-connect-router" {
		t.Fatalf("ImageKtRouter = %q", ImageKtRouter)
	}
}

func TestImageBuildConfigMatchesDefaultKtImages(t *testing.T) {
	root := findRepoRoot(t)

	releaser := readRepoFile(t, root, ".goreleaser.yml")
	assertContains(t, releaser, ImageKtShadow+":{{ .Tag }}")
	assertContains(t, releaser, ImageKtShadow+":v{{ .Major }}.{{ .Minor }}")
	assertContains(t, releaser, ImageKtRouter+":{{ .Tag }}")
	assertContains(t, releaser, ImageKtRouter+":v{{ .Major }}.{{ .Minor }}")

	shadowDockerfile := readRepoFile(t, root, "build", "docker", "shadow", "Dockerfile")
	assertContains(t, shadowDockerfile, "sed -i 's/\\r$//' /run.sh /disconnect.sh")

	makefile := readRepoFile(t, root, "Makefile")
	assertContains(t, makefile, "PREFIX			  ?= registry.cn-chengdu.aliyuncs.com/zodancer")
	assertContains(t, makefile, "CGO_ENABLED=0 GOARCH=amd64 GOOS=linux go build")
	assertContains(t, makefile, "push-shadow:")
	assertContains(t, makefile, "docker push $(PREFIX)/$(SHADOW_IMAGE):$(TAG)")
	assertContains(t, makefile, "push-router:")
	assertContains(t, makefile, "docker push $(PREFIX)/$(ROUTER_IMAGE):$(TAG)")

	releaseWorkflow := readRepoFile(t, root, ".github", "workflows", "release.yml")
	assertContains(t, releaseWorkflow, "login-server: registry.cn-chengdu.aliyuncs.com")
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".goreleaser.yml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("failed to find repository root")
		}
		dir = parent
	}
}

func readRepoFile(t *testing.T, root string, parts ...string) string {
	t.Helper()

	path := filepath.Join(append([]string{root}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func assertContains(t *testing.T, content, expected string) {
	t.Helper()

	if !strings.Contains(content, expected) {
		t.Fatalf("expected build config to contain %q", expected)
	}
}
