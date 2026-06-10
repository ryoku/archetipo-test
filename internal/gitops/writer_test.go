package gitops

import (
	"context"
	"errors"
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

const seedKustomization = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
  - name: gcr.io/test-project/test-service
    newTag: v1.0.0
resources:
  - deployment.yaml
`

func TestWriter_HappyPath(t *testing.T) {
	repoURL, bareDir := makeLocalBareRepo(t, map[string]string{
		"overlays/prod/kustomization.yaml": seedKustomization,
	})

	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := w.Apply(context.Background(), ApplyParams{
		OverlayPath:   "overlays/prod/kustomization.yaml",
		ImageName:     "gcr.io/test-project/test-service",
		NewTag:        "v2.0.0",
		ProductSlug:   "my-product",
		ComponentSlug: "test-service",
		EnvName:       "production",
		Actor:         "sara@example.com",
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

	tree, err := commit.Tree()
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}
	f, err := tree.File("overlays/prod/kustomization.yaml")
	if err != nil {
		t.Fatalf("Tree.File: %v", err)
	}
	contents, err := f.Contents()
	if err != nil {
		t.Fatalf("Contents: %v", err)
	}
	if !strings.Contains(contents, "v2.0.0") {
		t.Errorf("overlay missing new tag: %s", contents)
	}
	if !strings.Contains(contents, "apiVersion") {
		t.Errorf("overlay missing preserved key 'apiVersion': %s", contents)
	}
	if strings.Contains(contents, "v1.0.0") {
		t.Errorf("overlay still contains old tag: %s", contents)
	}
}

func TestWriter_OverlayNotFound(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		"overlays/prod/kustomization.yaml": seedKustomization,
	})

	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = w.Apply(context.Background(), ApplyParams{
		OverlayPath:   "overlays/nonexistent/kustomization.yaml",
		ImageName:     "gcr.io/test-project/test-service",
		NewTag:        "v2.0.0",
		ProductSlug:   "p",
		ComponentSlug: "c",
		EnvName:       "e",
		Actor:         "user",
	})

	var notFound *OverlayNotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *OverlayNotFoundError, got %T: %v", err, err)
	}
	if notFound.Path != "overlays/nonexistent/kustomization.yaml" {
		t.Errorf("Path = %q, want %q", notFound.Path, "overlays/nonexistent/kustomization.yaml")
	}
}

func TestWriter_Cleanup(t *testing.T) {
	repoURL, _ := makeLocalBareRepo(t, map[string]string{
		"overlays/prod/kustomization.yaml": seedKustomization,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	before := kubegateGitopsDirCount(t)

	_ = w.Apply(context.Background(), ApplyParams{
		OverlayPath:   "overlays/prod/kustomization.yaml",
		ImageName:     "gcr.io/test-project/test-service",
		NewTag:        "v2.0.0",
		ProductSlug:   "p",
		ComponentSlug: "c",
		EnvName:       "e",
		Actor:         "user",
	})
	if got := kubegateGitopsDirCount(t); got != before {
		t.Errorf("temp dir leaked after success: count before=%d, after=%d", before, got)
	}

	_ = w.Apply(context.Background(), ApplyParams{
		OverlayPath:   "does/not/exist.yaml",
		ImageName:     "gcr.io/test-project/test-service",
		NewTag:        "v2.0.0",
		ProductSlug:   "p",
		ComponentSlug: "c",
		EnvName:       "e",
		Actor:         "user",
	})
	if got := kubegateGitopsDirCount(t); got != before {
		t.Errorf("temp dir leaked after error: count before=%d, after=%d", before, got)
	}
}

func TestNewWriterFromEnv(t *testing.T) {
	t.Run("reads all fields", func(t *testing.T) {
		t.Setenv("GITOPS_REPO_URL", "https://git.example.com/repo.git")
		t.Setenv("GITOPS_TOKEN", "mytoken")
		t.Setenv("GITOPS_DEPLOY_KEY_PATH", "")

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
		"overlays/prod/kustomization.yaml": seedKustomization,
	})
	w, err := New(WriterConfig{RepoURL: repoURL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p := ApplyParams{
		OverlayPath:   "overlays/prod/kustomization.yaml",
		ImageName:     "gcr.io/test-project/test-service",
		NewTag:        "v1.0.0", // same as the seed tag — no change
		ProductSlug:   "p",
		ComponentSlug: "c",
		EnvName:       "e",
		Actor:         "user",
	}
	if err := w.Apply(context.Background(), p); err != nil {
		t.Errorf("Apply with same tag should return nil, got: %v", err)
	}
}

func TestOverlayNotFoundError_Message(t *testing.T) {
	err := &OverlayNotFoundError{Path: "overlays/prod/kustomization.yaml"}
	want := "gitops writer: overlay file not found: overlays/prod/kustomization.yaml"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
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
	err = w.Apply(context.Background(), ApplyParams{
		OverlayPath:   "overlays/kustomization.yaml",
		ImageName:     "gcr.io/p/s",
		NewTag:        "v1",
		ProductSlug:   "p",
		ComponentSlug: "s",
		EnvName:       "e",
		Actor:         "user",
	})
	if err == nil {
		t.Fatal("expected error loading nonexistent SSH key")
	}
	if !strings.Contains(err.Error(), "load SSH key") {
		t.Errorf("error = %q, want it to contain 'load SSH key'", err.Error())
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
