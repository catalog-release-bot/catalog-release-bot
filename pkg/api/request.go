package api

import (
	"fmt"

	"go.uber.org/multierr"
)

type Request struct {
	CatalogBranch     string            `json:"catalogBranch"`
	GithubToken       string            `json:"githubToken"`
	PackageName       string            `json:"packageName"`
	PackageRepoOwner  string            `json:"packageRepoOwner"`
	PackageRepoName   string            `json:"packageRepoName"`
	PackageCommitHash string            `json:"packageCommitHash"`
	PackageData       map[string][]byte `json:"packageData"`
}

func (r Request) Validate() error {
	errs := []error{}
	if r.CatalogBranch == "" {
		errs = append(errs, fmt.Errorf("catalogBranch is undefined"))
	}
	if r.GithubToken == "" {
		errs = append(errs, fmt.Errorf("githubToken is undefined"))
	}
	if r.PackageName == "" {
		errs = append(errs, fmt.Errorf("packageName is undefined"))
	}
	if r.PackageRepoOwner == "" {
		errs = append(errs, fmt.Errorf("packageRepoOwner is undefined"))
	}
	if r.PackageRepoName == "" {
		errs = append(errs, fmt.Errorf("packageRepoName is undefined"))
	}
	if r.PackageCommitHash == "" {
		errs = append(errs, fmt.Errorf("packageCommitHash is undefined"))
	}
	if len(r.PackageData) == 0 {
		errs = append(errs, fmt.Errorf("packageData is undefined"))
	}
	return multierr.Combine(errs...)
}
