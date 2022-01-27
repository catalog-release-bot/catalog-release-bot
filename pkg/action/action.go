package action

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"github.com/catalog-release-bot/catalog-release-bot/pkg/api"
)

type Action struct {
	ActionWebhookURL string
	Catalog          fs.FS

	GithubToken       string
	CatalogBranch     string
	PackageName       string
	PackageRepoOwner  string
	PackageRepoName   string
	PackageCommitHash string
}

func (a *Action) Run(ctx context.Context) error {
	req := api.Request{
		CatalogBranch:     a.CatalogBranch,
		GithubToken:       a.GithubToken,
		PackageName:       a.PackageName,
		PackageRepoOwner:  a.PackageRepoOwner,
		PackageRepoName:   a.PackageRepoName,
		PackageCommitHash: a.PackageCommitHash,
		PackageData:       map[string][]byte{},
	}

	if err := fs.WalkDir(a.Catalog, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		filedata, err := fs.ReadFile(a.Catalog, path)
		if err != nil {
			return err
		}
		req.PackageData[path] = filedata
		return nil
	}); err != nil {
		return err
	}
	reqData, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(a.ActionWebhookURL, "application/json", bytes.NewReader(reqData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("webhook failed (%s): %q", resp.Status, string(respBody))
	}
	return nil
}
