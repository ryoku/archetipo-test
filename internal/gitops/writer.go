package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// WriterConfig holds the configuration for the gitops Writer.
type WriterConfig struct {
	RepoURL       string
	DeployKeyPath string // SSH key path; mutually exclusive with Token
	Token         string // HTTPS personal access token; mutually exclusive with DeployKeyPath
}

// ApplyParams are the parameters for a single gitops write operation.
type ApplyParams struct {
	OverlayPath   string // relative path within the repo to kustomization.yaml
	ImageName     string // full GCR image path (from Component.GCRImagePath)
	NewTag        string // image tag to deploy
	ProductSlug   string
	ComponentSlug string
	EnvName       string
	Actor         string // authenticated user identifier; appears in the commit message
}

// OverlayNotFoundError is returned when the overlay file does not exist at the expected path.
type OverlayNotFoundError struct {
	Path string
}

func (e *OverlayNotFoundError) Error() string {
	return fmt.Sprintf("gitops writer: overlay file not found: %s", e.Path)
}

// Writer applies gitops write operations: clone → patch → commit → push.
type Writer struct {
	cfg WriterConfig
}

// New validates cfg and returns a Writer. RepoURL is required; at most one auth method may be set.
func New(cfg WriterConfig) (*Writer, error) {
	if cfg.RepoURL == "" {
		return nil, fmt.Errorf("gitops writer: RepoURL is required")
	}
	if cfg.DeployKeyPath != "" && cfg.Token != "" {
		return nil, fmt.Errorf("gitops writer: DeployKeyPath and Token are mutually exclusive")
	}
	return &Writer{cfg: cfg}, nil
}

// NewWriterFromEnv reads GITOPS_REPO_URL, GITOPS_DEPLOY_KEY_PATH, and GITOPS_TOKEN from
// the environment and delegates to New.
func NewWriterFromEnv() (*Writer, error) {
	return New(WriterConfig{
		RepoURL:       os.Getenv("GITOPS_REPO_URL"),
		DeployKeyPath: os.Getenv("GITOPS_DEPLOY_KEY_PATH"),
		Token:         os.Getenv("GITOPS_TOKEN"),
	})
}

// Apply clones the gitops repo into a temp directory, patches the Kustomize overlay file,
// commits with a structured message, pushes, and removes the temp directory.
// The temp directory is always removed, even on error.
func (w *Writer) Apply(ctx context.Context, p ApplyParams) error {
	tmpDir, err := os.MkdirTemp("", "kubegate-gitops-*")
	if err != nil {
		return fmt.Errorf("gitops writer: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	auth, err := w.buildAuth()
	if err != nil {
		return err
	}

	repo, err := git.PlainCloneContext(ctx, tmpDir, false, &git.CloneOptions{
		URL:  w.cfg.RepoURL,
		Auth: auth,
	})
	if err != nil {
		return fmt.Errorf("gitops writer: clone: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("gitops writer: get worktree: %w", err)
	}

	overlayAbs := filepath.Join(tmpDir, filepath.FromSlash(p.OverlayPath))
	data, err := os.ReadFile(overlayAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return &OverlayNotFoundError{Path: p.OverlayPath}
		}
		return fmt.Errorf("gitops writer: read overlay: %w", err)
	}

	patched, err := PatchImage(data, p.ImageName, p.NewTag)
	if err != nil {
		return fmt.Errorf("gitops writer: patch overlay: %w", err)
	}

	if err := os.WriteFile(overlayAbs, patched, 0644); err != nil {
		return fmt.Errorf("gitops writer: write overlay: %w", err)
	}

	if _, err := worktree.Add(filepath.ToSlash(p.OverlayPath)); err != nil {
		return fmt.Errorf("gitops writer: stage overlay: %w", err)
	}

	msg := fmt.Sprintf("deploy(%s/%s/%s): %s by %s",
		p.ProductSlug, p.ComponentSlug, p.EnvName, p.NewTag, p.Actor)

	if _, err = worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "KubeGate",
			Email: "kubegate@noreply",
			When:  time.Now(),
		},
	}); err != nil {
		if errors.Is(err, git.ErrEmptyCommit) {
			// Tag is already deployed; treat as a no-op rather than an error.
			return nil
		}
		return fmt.Errorf("gitops writer: commit: %w", err)
	}

	if err := repo.PushContext(ctx, &git.PushOptions{Auth: auth}); err != nil {
		if errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil
		}
		return fmt.Errorf("gitops writer: push: %w", err)
	}

	return nil
}

func (w *Writer) buildAuth() (transport.AuthMethod, error) {
	if w.cfg.DeployKeyPath != "" {
		sshAuth, err := gitssh.NewPublicKeysFromFile("git", w.cfg.DeployKeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("gitops writer: load SSH key: %w", err)
		}
		sshAuth.HostKeyCallbackHelper = gitssh.HostKeyCallbackHelper{
			HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		}
		return sshAuth, nil
	}
	if w.cfg.Token != "" {
		return &githttp.BasicAuth{
			Username: "x-token-auth",
			Password: w.cfg.Token,
		}, nil
	}
	return nil, nil
}
