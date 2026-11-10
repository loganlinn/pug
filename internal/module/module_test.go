package module

import (
	"context"
	"os"
	"testing"

	"github.com/leg100/pug/internal"
	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	os.MkdirAll("./testdata/modules/with_both_s3_backend_and_dot_terraform_dir/.terraform", 0o755)

	got := New(Options{Path: "with_s3_backend", Backend: "s3"})
	assert.Equal(t, "with_s3_backend", got.Path)
}

func TestFindModules(t *testing.T) {
	workdir, _ := internal.NewWorkdir("./testdata/modules")
	modules, errch := find(context.Background(), workdir, []string{"**/*"}, []string{"**/.terraform"})

	var got []Options
	for opts := range modules {
		got = append(got, opts)
	}

	assert.Equal(t, 6, len(got), got)
	assert.Contains(t, got, Options{Path: "with_local_backend", Backend: "local"})
	assert.Contains(t, got, Options{Path: "with_s3_backend", Backend: "s3"})
	assert.Contains(t, got, Options{Path: "with_cloud_backend", Backend: "cloud"})
	assert.Contains(t, got, Options{Path: "terragrunt_with_local", Backend: "local"})
	assert.Contains(t, got, Options{Path: "terragrunt_without_backend", Backend: ""})
	assert.Contains(t, got, Options{Path: "multiple_tf_files", Backend: "local"})
	assert.NotContains(t, got, "broken")

	// Expect one error from broken module then error channel should close
	goterr := <-errch
	assert.Contains(t, goterr.Error(), "Unclosed configuration block")
	_, closed := <-errch
	assert.False(t, closed)
}

func TestFindModules_IncludeExclude(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		want     []string
	}{
		{
			name:     "exclude",
			includes: []string{"**/*"},
			excludes: []string{"with_*"},
			want:     []string{"terragrunt_with_local", "terragrunt_without_backend", "multiple_tf_files"},
		},
		{
			name:     "include",
			includes: []string{"terragrunt_*"},
			want:     []string{"terragrunt_with_local", "terragrunt_without_backend"},
		},
		{
			name:     "exclude takes precedence over include",
			includes: []string{"terragrunt_*"},
			excludes: []string{"*_without_*"},
			want:     []string{"terragrunt_with_local"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workdir, _ := internal.NewWorkdir("./testdata/modules")
			modules, _ := find(context.Background(), workdir, tt.includes, tt.excludes)

			var got []string
			for opts := range modules {
				got = append(got, opts.Path)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

func TestMatches(t *testing.T) {
	tests := []struct {
		pattern string
		dir     string
		want    bool
	}{
		{"**/*", ".", true},
		{"**/*", "a/b", true},
		{"live", "live", true},
		{"live", "live/prod/app", true},
		{"live", "lively", false},
		{"live/*", "live", false},
		{"live/*", "live/prod/app", true},
		{"**/.terraform", ".terraform", true},
		{"**/.terraform", "a/.terraform/modules/b", true},
		{"**/examples", "modules/vpc/examples/simple", true},
		{"**/examples", "modules/vpc", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, matches([]string{tt.pattern}, tt.dir), "pattern %q, dir %q", tt.pattern, tt.dir)
	}
}
