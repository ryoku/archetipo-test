package gitops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/skeema/knownhosts"
	gossh "golang.org/x/crypto/ssh"
)

// WriterConfig holds the configuration for the gitops Writer.
// Exactly one of DeployKeyPath (SSH) or Token (HTTPS) may be set; both empty means no auth
// (suitable for local file:// remotes used in tests).
// When DeployKeyPath is set, KnownHostsPath must also be set for SSH host key verification.
// Leaving KnownHostsPath empty disables host key verification — only acceptable for
// local dev against a file:// remote.
type WriterConfig struct {
	RepoURL        string
	DeployKeyPath  string // SSH key path; mutually exclusive with Token
	KnownHostsPath string // SSH known_hosts file; required when DeployKeyPath is set for host verification
	Token          string // HTTPS personal access token; mutually exclusive with DeployKeyPath
}

// ApplyParams are the parameters for a single gitops write operation.
type ApplyParams struct {
	OverlayPath   string // relative path within the repo to the overlay file containing the Kustomize images list
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

// NewWriterFromEnv reads GITOPS_REPO_URL, GITOPS_DEPLOY_KEY_PATH, GITOPS_KNOWN_HOSTS_PATH,
// and GITOPS_TOKEN from the environment and calls New.
// If both credential variables are empty, the Writer operates in unauthenticated mode
// (suitable for file:// remotes). Returns an error if GITOPS_REPO_URL is empty.
func NewWriterFromEnv() (*Writer, error) {
	return New(WriterConfig{
		RepoURL:        os.Getenv("GITOPS_REPO_URL"),
		DeployKeyPath:  os.Getenv("GITOPS_DEPLOY_KEY_PATH"),
		KnownHostsPath: os.Getenv("GITOPS_KNOWN_HOSTS_PATH"),
		Token:          os.Getenv("GITOPS_TOKEN"),
	})
}

// Apply clones the gitops repo into a temp directory, patches the Kustomize overlay file,
// commits with a structured message, pushes, and removes the temp directory.
// The temp directory is always removed, even on error.
//
// Each call performs a fresh clone rather than reusing a cached local copy; this avoids
// stale-state bugs at the cost of a full clone per deployment, which is acceptable for
// the current usage pattern (one deploy at a time, advisory locking handled by callers).
//
// IMPORTANT: This function is not safe for concurrent use against the same branch. Callers
// must hold a deployment lock (e.g. a PostgreSQL advisory lock on the product+environment)
// before calling Apply to prevent non-fast-forward push failures.
func (w *Writer) Apply(ctx context.Context, p ApplyParams) error {
	if p.OverlayPath == "" {
		return fmt.Errorf("gitops writer: OverlayPath must not be empty")
	}
	if p.ImageName == "" {
		return fmt.Errorf("gitops writer: ImageName must not be empty")
	}
	if p.NewTag == "" {
		return fmt.Errorf("gitops writer: NewTag must not be empty")
	}
	if p.Actor == "" {
		return fmt.Errorf("gitops writer: Actor must not be empty")
	}

	tmpDir, err := os.MkdirTemp("", "kubegate-gitops-*")
	if err != nil {
		return fmt.Errorf("gitops writer: create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck // best-effort cleanup of temp clone; errors here are non-actionable

	auth, err := w.buildAuth()
	if err != nil {
		return fmt.Errorf("gitops writer: build auth: %w", err)
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

	if err := patchAndStage(tmpDir, worktree, p); err != nil {
		return err
	}

	msg := fmt.Sprintf("deploy(%s/%s/%s): %s by %s",
		p.ProductSlug, p.ComponentSlug, p.EnvName, p.NewTag, p.Actor)

	return commitAndPush(ctx, repo, worktree, auth, msg)
}

// patchAndStage reads the overlay file, applies the image patch, writes it back, and stages it.
func patchAndStage(tmpDir string, worktree *git.Worktree, p ApplyParams) error {
	overlayAbs, err := securejoin.SecureJoin(tmpDir, p.OverlayPath)
	if err != nil {
		return fmt.Errorf("gitops writer: unsafe overlay path: %w", err)
	}
	data, err := os.ReadFile(overlayAbs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
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
	return nil
}

// commitAndPush creates a commit authored by the KubeGate system identity and pushes it.
// ErrEmptyCommit (no staged changes — e.g. overlay content already matches the requested tag)
// and NoErrAlreadyUpToDate are treated as no-ops.
func commitAndPush(ctx context.Context, repo *git.Repository, worktree *git.Worktree, auth transport.AuthMethod, msg string) error {
	_, err := worktree.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "KubeGate",
			Email: "noreply@kubegate.local",
			When:  time.Now(),
		},
	})
	if err != nil {
		if errors.Is(err, git.ErrEmptyCommit) {
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

// buildAuth returns the transport.AuthMethod derived from WriterConfig.
// SSH: loads the private key and verifies host keys against KnownHostsPath when set.
// If KnownHostsPath is empty, host key verification is skipped — only acceptable for
// local file:// remotes (tests/dev); never use in production against a real remote.
// HTTPS: uses Token as a bearer credential.
// No auth fields set: returns nil (unauthenticated, suitable for file:// remotes).
func (w *Writer) buildAuth() (transport.AuthMethod, error) {
	if w.cfg.DeployKeyPath != "" {
		sshAuth, err := gitssh.NewPublicKeysFromFile("git", w.cfg.DeployKeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("load SSH key: %w", err)
		}
		if w.cfg.KnownHostsPath != "" {
			cb, err := knownhosts.New(w.cfg.KnownHostsPath)
			if err != nil {
				return nil, fmt.Errorf("load known_hosts: %w", err)
			}
			sshAuth.HostKeyCallbackHelper = gitssh.HostKeyCallbackHelper{HostKeyCallback: gossh.HostKeyCallback(cb)}
		} else {
			// KnownHostsPath not set: skip host key verification.
			// Acceptable only for local file:// remotes; do not use against a real SSH remote.
			sshAuth.HostKeyCallbackHelper = gitssh.HostKeyCallbackHelper{
				HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec
			}
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
