package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// makeLocalBareRepo creates a non-bare repo seeded with files, commits them, then
// bare-clones it to serve as the test "remote". Returns the file:// URL and the bare
// repo's directory path (for post-call verification).
func makeLocalBareRepo(t *testing.T, files map[string]string) (repoURL string, bareDir string) {
	t.Helper()

	srcDir := t.TempDir()
	srcRepo, err := git.PlainInit(srcDir, false)
	if err != nil {
		t.Fatalf("PlainInit: %v", err)
	}
	wt, err := srcRepo.Worktree()
	if err != nil {
		t.Fatalf("Worktree: %v", err)
	}
	for path, content := range files {
		abs := filepath.Join(srcDir, filepath.FromSlash(path))
		if mkErr := os.MkdirAll(filepath.Dir(abs), 0755); mkErr != nil {
			t.Fatalf("MkdirAll %s: %v", filepath.Dir(abs), mkErr)
		}
		if wErr := os.WriteFile(abs, []byte(content), 0644); wErr != nil {
			t.Fatalf("WriteFile %s: %v", abs, wErr)
		}
		if _, addErr := wt.Add(filepath.ToSlash(path)); addErr != nil {
			t.Fatalf("Add %s: %v", path, addErr)
		}
	}
	if _, cErr := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{Name: "seed", Email: "seed@test", When: time.Now()},
	}); cErr != nil {
		t.Fatalf("Commit: %v", cErr)
	}

	bareDir = t.TempDir()
	if _, clErr := git.PlainClone(bareDir, true, &git.CloneOptions{URL: srcDir}); clErr != nil {
		t.Fatalf("PlainClone bare: %v", clErr)
	}
	return "file://" + bareDir, bareDir
}

// kubegateGitopsDirCount returns the number of kubegate-gitops-* directories in os.TempDir().
func kubegateGitopsDirCount(t *testing.T) int {
	t.Helper()
	entries, _ := os.ReadDir(os.TempDir())
	n := 0
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), "kubegate-gitops-") {
			n++
		}
	}
	return n
}

const seedHelmRelease = `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-product
spec:
  values:
    test-service:
      image:
        repository: gcr.io/test-project/test-service
        tag: v1.0.0
`

const helmReleasePath = "apps/production/my-product/my-product-helmrelease.yaml"

func TestWriter_HappyPath(t *testing.T) {
	repoURL, bareDir := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})

	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        "test-service",
		NewTag:          "v2.0.0",
		ProductSlug:     "my-product",
		EnvName:         "production",
		Actor:           "sara@example.com",
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	bareRepo, err := git.PlainOpen(bareDir)
	if err != nil {
		t.Fatalf("PlainOpen bare: %v", err)
	}
	ref, err := bareRepo.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	commit, err := bareRepo.CommitObject(ref.Hash())
	if err != nil {
		t.Fatalf("CommitObject: %v", err)
	}

	wantMsg := "deploy(my-product/test-service/production): v2.0.0 by sara@example.com"
	if got := strings.TrimSpace(commit.Message); got != wantMsg {
		t.Errorf("commit message = %q, want %q", got, wantMsg)
	}
	if commit.Author.Name != "KubeGate" {
		t.Errorf("author name = %q, want %q", commit.Author.Name, "KubeGate")
	}
	if commit.Author.Email != "noreply@kubegate.local" {
		t.Errorf("author email = %q, want %q", commit.Author.Email, "noreply@kubegate.local")
	}

	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	f, err := tree.File(helmReleasePath)
	if err != nil {
		t.Fatalf("Tree.File: %v", err)
	}
	contents, err := f.Contents()
	if err != nil {
		t.Fatalf("Contents: %v", err)
	}
	if !strings.Contains(contents, "v2.0.0") {
		t.Errorf("helmrelease missing new tag: %s", contents)
	}
	if !strings.Contains(contents, "apiVersion") {
		t.Errorf("helmrelease missing preserved key 'apiVersion': %s", contents)
	}
	if strings.Contains(contents, "v1.0.0") {
		t.Errorf("helmrelease still contains old tag: %s", contents)
	}
}

func TestWriter_HelmReleaseNotFound(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})

	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	missingPath := "apps/production/other/other-helmrelease.yaml"
	_, err = w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: missingPath,
		Workload:        "svc",
		NewTag:          "v2.0.0",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	})

	var notFound *HelmReleaseNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *HelmReleaseNotFoundError, got %T: %v", err, err)
	}
	if notFound.Path != missingPath {
		t.Errorf("Path = %q, want %q", notFound.Path, missingPath)
	}
}

func TestWriter_Cleanup(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	before := kubegateGitopsDirCount(t)

	_, _ = w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        "test-service",
		NewTag:          "v2.0.0",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	})
	if got := kubegateGitopsDirCount(t); got != before {
		t.Errorf("temp dir leaked after success: count before=%d, after=%d", before, got)
	}

	_, _ = w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: "apps/production/does/not/exist.yaml",
		Workload:        "svc",
		NewTag:          "v2.0.0",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	})
	if got := kubegateGitopsDirCount(t); got != before {
		t.Errorf("temp dir leaked after Apply error: count before=%d, after=%d", before, got)
	}

	// ListWorkloads uses a separate kubegate-gitops-read-* prefix — verify no leak there either.
	_, _ = w.ListWorkloads(context.Background(), "my-product", "production")
	_, _ = w.ListWorkloads(context.Background(), "missing-product", "production")
	if got := kubegateGitopsDirCount(t); got != before {
		t.Errorf("temp dir leaked after ListWorkloads: count before=%d, after=%d", before, got)
	}
}

func TestNewWriterFromEnv(t *testing.T) {
	t.Run("reads HTTPS token fields", func(t *testing.T) {
		t.Setenv("GITOPS_REPO_URL", "https://git.example.com/repo.git")
		t.Setenv("GITOPS_TOKEN", "mytoken")
		t.Setenv("GITOPS_DEPLOY_KEY_PATH", "")
		t.Setenv("GITOPS_KNOWN_HOSTS_PATH", "")

		w, err := NewWriterFromEnv()
		if err != nil {
			t.Fatalf("NewWriterFromEnv: %v", err)
		}
		if w.cfg.RepoURL != "https://git.example.com/repo.git" {
			t.Errorf("RepoURL = %q", w.cfg.RepoURL)
		}
		if w.cfg.Token != "mytoken" {
			t.Errorf("Token = %q", w.cfg.Token)
		}
	})

	t.Run("reads SSH key fields", func(t *testing.T) {
		t.Setenv("GITOPS_REPO_URL", "git@example.com:org/repo.git")
		t.Setenv("GITOPS_DEPLOY_KEY_PATH", "/home/user/.ssh/id_ed25519")
		t.Setenv("GITOPS_KNOWN_HOSTS_PATH", "/home/user/.ssh/known_hosts")
		t.Setenv("GITOPS_TOKEN", "")

		w, err := NewWriterFromEnv()
		if err != nil {
			t.Fatalf("NewWriterFromEnv: %v", err)
		}
		if w.cfg.DeployKeyPath != "/home/user/.ssh/id_ed25519" {
			t.Errorf("DeployKeyPath = %q", w.cfg.DeployKeyPath)
		}
		if w.cfg.KnownHostsPath != "/home/user/.ssh/known_hosts" {
			t.Errorf("KnownHostsPath = %q", w.cfg.KnownHostsPath)
		}
	})

	t.Run("error when GITOPS_REPO_URL is empty", func(t *testing.T) {
		t.Setenv("GITOPS_REPO_URL", "")
		t.Setenv("GITOPS_TOKEN", "")
		t.Setenv("GITOPS_DEPLOY_KEY_PATH", "")

		if _, err := NewWriterFromEnv(); err == nil {
			t.Fatal("expected error when GITOPS_REPO_URL is empty")
		}
	})
}

func TestWriter_IdempotentRedeployNoError(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p := ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        "test-service",
		NewTag:          "v1.0.0", // same as the seed tag — no change
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	}
	if _, err := w.Apply(context.Background(), p); err != nil {
		t.Errorf("Apply with same tag should return nil, got: %v", err)
	}
}

func TestHelmReleaseNotFoundError_Message(t *testing.T) {
	err := &HelmReleaseNotFoundError{Path: "apps/prod/svc/svc-helmrelease.yaml"}
	want := "gitops writer: HelmRelease not found: apps/prod/svc/svc-helmrelease.yaml"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestWriter_PathTraversalRejected(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: "../../etc/hostname",
		Workload:        "svc",
		NewTag:          "v2.0.0",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	})
	// securejoin should either return an error or confine the path inside tmpDir,
	// resulting in a HelmReleaseNotFoundError — either way Apply must not succeed.
	if err == nil {
		t.Fatal("expected error for path traversal HelmReleasePath, got nil")
	}
}

func TestApply_RequiredParamValidation(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	base := ApplyParams{
		HelmReleasePath: helmReleasePath,
		Workload:        "test-service",
		NewTag:          "v2.0.0",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	}
	cases := []struct {
		name    string
		mutate  func(*ApplyParams)
		wantMsg string
	}{
		{"empty HelmReleasePath", func(p *ApplyParams) { p.HelmReleasePath = "" }, "HelmReleasePath must not be empty"},
		{"empty Workload", func(p *ApplyParams) { p.Workload = "" }, "Workload must not be empty"},
		{"empty NewTag", func(p *ApplyParams) { p.NewTag = "" }, "NewTag must not be empty"},
		{"empty Actor", func(p *ApplyParams) { p.Actor = "" }, "Actor must not be empty"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := base
			tc.mutate(&p)
			_, err := w.Apply(context.Background(), p)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantMsg)
			}
		})
	}
}

func TestNew_SSHKeyPathNotFound(t *testing.T) {
	w, err := New(WriterConfig{
		RepoURL:       "git@example.com:org/repo.git",
		DeployKeyPath: "/nonexistent/key",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// buildAuth is called inside Apply; trigger it directly via Apply to cover the error path.
	_, err = w.Apply(context.Background(), ApplyParams{
		HelmReleasePath: "apps/e/p/p-helmrelease.yaml",
		Workload:        "s",
		NewTag:          "v1",
		ProductSlug:     "p",
		EnvName:         "e",
		Actor:           "user",
	})
	if err == nil {
		t.Fatal("expected error loading nonexistent SSH key")
	}
	if !strings.Contains(err.Error(), "build auth") {
		t.Errorf("error = %q, want it to contain 'build auth'", err.Error())
	}
}

func TestNew_MutuallyExclusiveAuth(t *testing.T) {
	_, err := New(WriterConfig{
		RepoURL:       "https://example.com/repo.git",
		DeployKeyPath: "/path/to/key",
		Token:         "mytoken",
	})
	if err == nil {
		t.Fatal("expected error when both DeployKeyPath and Token are set")
	}
}

func TestWriter_ListWorkloads_HappyPath(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	workloads, err := w.ListWorkloads(context.Background(), "my-product", "production")
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload, got %d", len(workloads))
	}
	if workloads[0].Name != "test-service" {
		t.Errorf("workload name = %q, want %q", workloads[0].Name, "test-service")
	}
	if workloads[0].ImageRepository != "gcr.io/test-project/test-service" {
		t.Errorf("image repository = %q, want %q", workloads[0].ImageRepository, "gcr.io/test-project/test-service")
	}
}

func TestWriter_ListWorkloads_HelmReleaseNotFound(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = w.ListWorkloads(context.Background(), "other-product", "production")
	if !errors.Is(err, ErrHelmReleaseNotFound) {
		t.Fatalf("expected ErrHelmReleaseNotFound, got %T: %v", err, err)
	}
}

func TestWriter_ListWorkloads_ParseError(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: "not: valid: yaml: ::::",
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = w.ListWorkloads(context.Background(), "my-product", "production")
	if !errors.Is(err, ErrHelmReleaseParseFailed) {
		t.Fatalf("expected ErrHelmReleaseParseFailed, got %T: %v", err, err)
	}
}

func TestWriter_ReadCurrentTags_HappyPath(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tags, err := w.ReadCurrentTags(context.Background(), "my-product", "production")
	if err != nil {
		t.Fatalf("ReadCurrentTags: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag entry, got %d: %v", len(tags), tags)
	}
	if tags["test-service"] != "v1.0.0" {
		t.Errorf("test-service tag = %q, want %q", tags["test-service"], "v1.0.0")
	}
}

func TestWriter_ReadCurrentTags_HelmReleaseNotFound(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: seedHelmRelease,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = w.ReadCurrentTags(context.Background(), "other-product", "production")
	if !errors.Is(err, ErrHelmReleaseNotFound) {
		t.Fatalf("expected ErrHelmReleaseNotFound, got %T: %v", err, err)
	}
}

func TestWriter_ReadCurrentTags_NoTagReturnsNA(t *testing.T) {
	const helmReleaseNoTag = `apiVersion: helm.toolkit.fluxcd.io/v2beta1
kind: HelmRelease
metadata:
  name: my-product
spec:
  values:
    test-service:
      image:
        repository: gcr.io/test-project/test-service
`
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		helmReleasePath: helmReleaseNoTag,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	tags, err := w.ReadCurrentTags(context.Background(), "my-product", "production")
	if err != nil {
		t.Fatalf("ReadCurrentTags: %v", err)
	}
	if tags["test-service"] != "N/A" {
		t.Errorf("test-service tag = %q, want %q", tags["test-service"], "N/A")
	}
}

func TestHelmReleaseNotFoundError_Unwrap(t *testing.T) {
	err := &HelmReleaseNotFoundError{Path: "apps/production/svc/svc-helmrelease.yaml"}
	if !errors.Is(err, ErrHelmReleaseNotFound) {
		t.Errorf("errors.Is via Unwrap should match ErrHelmReleaseNotFound, got false")
	}
	wrapped := fmt.Errorf("outer: %w", err)
	if !errors.Is(wrapped, ErrHelmReleaseNotFound) {
		t.Errorf("errors.Is through wrapping should match ErrHelmReleaseNotFound, got false")
	}
	var target *HelmReleaseNotFoundError
	if !errors.As(wrapped, &target) {
		t.Errorf("errors.As should extract *HelmReleaseNotFoundError from wrapped error, got false")
	}
	if target.Path != err.Path {
		t.Errorf("extracted Path = %q, want %q", target.Path, err.Path)
	}
}
