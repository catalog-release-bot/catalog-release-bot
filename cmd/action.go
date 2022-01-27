package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/catalog-release-bot/catalog-release-bot/pkg/action"
)

func newActionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "action",
		Run: func(cmd *cobra.Command, args []string) {
			if os.Getenv("GITHUB_EVENT_NAME") != "push" || os.Getenv("GITHUB_REF_TYPE") != "branch" {
				log.Fatal("catalog-release-bot only supports branch pushes")
			}

			catalogDir := os.Getenv("CATALOG_DIR")
			if catalogDir == "" {
				log.Fatal("catalog directory undefined")
			}

			packageRepoOwner := os.Getenv("GITHUB_REPOSITORY_OWNER")
			packageRepoName := strings.TrimPrefix(os.Getenv("GITHUB_REPOSITORY"), fmt.Sprintf("%s/", packageRepoOwner))

			a := action.Action{
				ActionWebhookURL:  "https://catalog-release-bot-h4hjs7xpma-uk.a.run.app/webhook",
				Catalog:           os.DirFS(catalogDir),
				CatalogBranch:     strings.TrimPrefix(os.Getenv("GITHUB_REF"), "refs/heads/"),
				GithubToken:       os.Getenv("GITHUB_TOKEN"),
				PackageName:       os.Getenv("PACKAGE_NAME"),
				PackageRepoOwner:  packageRepoOwner,
				PackageRepoName:   packageRepoName,
				PackageCommitHash: os.Getenv("GITHUB_SHA"),
			}

			if err := a.Run(cmd.Context()); err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}
