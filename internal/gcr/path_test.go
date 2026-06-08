package gcr

import "testing"

func TestParseImagePath(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantProj   string
		wantLoc    string
		wantRepo   string
		wantPkg    string
		wantParent string
	}{
		{
			name:       "standard AR path",
			input:      "us-central1-docker.pkg.dev/my-project/my-repo/my-image",
			wantProj:   "my-project",
			wantLoc:    "us-central1",
			wantRepo:   "my-repo",
			wantPkg:    "my-image",
			wantParent: "projects/my-project/locations/us-central1/repositories/my-repo/packages/my-image",
		},
		{
			name:       "regional shorthand (us-docker.pkg.dev)",
			input:      "us-docker.pkg.dev/proj/repo/img",
			wantProj:   "proj",
			wantLoc:    "us",
			wantRepo:   "repo",
			wantPkg:    "img",
			wantParent: "projects/proj/locations/us/repositories/repo/packages/img",
		},
		{
			name:       "europe location",
			input:      "europe-west1-docker.pkg.dev/acme/backend/api-server",
			wantProj:   "acme",
			wantLoc:    "europe-west1",
			wantRepo:   "backend",
			wantPkg:    "api-server",
			wantParent: "projects/acme/locations/europe-west1/repositories/backend/packages/api-server",
		},
		{
			name:       "nested image name",
			input:      "us-docker.pkg.dev/proj/repo/ns/subimage",
			wantProj:   "proj",
			wantLoc:    "us",
			wantRepo:   "repo",
			wantPkg:    "ns/subimage",
			wantParent: "projects/proj/locations/us/repositories/repo/packages/ns/subimage",
		},
		{
			name:    "legacy gcr.io path",
			input:   "gcr.io/my-project/my-image",
			wantErr: true,
		},
		{
			name:    "legacy us.gcr.io path",
			input:   "us.gcr.io/my-project/my-image",
			wantErr: true,
		},
		{
			name:    "no slash",
			input:   "us-docker.pkg.dev",
			wantErr: true,
		},
		{
			name:    "wrong host suffix",
			input:   "us-central1.docker.io/proj/repo/img",
			wantErr: true,
		},
		{
			name:    "missing project segment",
			input:   "us-docker.pkg.dev/only-one-segment",
			wantErr: true,
		},
		{
			name:    "missing image segment",
			input:   "us-docker.pkg.dev/proj/repo",
			wantErr: true,
		},
		{
			name:    "empty path after host",
			input:   "us-docker.pkg.dev/",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseImagePath(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			assertParsedPath(t, got, tc.wantProj, tc.wantLoc, tc.wantRepo, tc.wantPkg, tc.wantParent)
		})
	}
}

func assertParsedPath(t *testing.T, got parsedPath, wantProj, wantLoc, wantRepo, wantPkg, wantParent string) {
	t.Helper()
	if got.project != wantProj {
		t.Errorf("project: got %q, want %q", got.project, wantProj)
	}
	if got.location != wantLoc {
		t.Errorf("location: got %q, want %q", got.location, wantLoc)
	}
	if got.repo != wantRepo {
		t.Errorf("repo: got %q, want %q", got.repo, wantRepo)
	}
	if got.pkg != wantPkg {
		t.Errorf("pkg: got %q, want %q", got.pkg, wantPkg)
	}
	if parent := got.resourceParent(); parent != wantParent {
		t.Errorf("resourceParent: got %q, want %q", parent, wantParent)
	}
}
