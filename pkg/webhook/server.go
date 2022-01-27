package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v42/github"

	"github.com/catalog-release-bot/catalog-release-bot/pkg/api"
	"github.com/catalog-release-bot/catalog-release-bot/pkg/gh"
)

type Server struct {
	Addr            string
	ShutdownTimeout time.Duration
}

func (s *Server) Run(ctx context.Context, handler http.Handler) error {
	if s.Addr == "" {
		s.Addr = "localhost:0"
	}
	srv := http.Server{
		Addr:    s.Addr,
		Handler: handler,
	}

	done := make(chan error, 1)
	go func() {
		cancelCtx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		defer cancel()
		<-cancelCtx.Done()

		shutdownCtx := context.Background()
		var shutdownCancel context.CancelFunc
		if s.ShutdownTimeout > 0 {
			shutdownCtx, shutdownCancel = context.WithTimeout(ctx, s.ShutdownTimeout)
			defer shutdownCancel()
		}

		log.Printf("shutting down")
		done <- srv.Shutdown(shutdownCtx)
	}()

	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}

	log.Printf("listening on http://%s/webhook", l.Addr())
	if err := srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return <-done
}

type Handler struct {
	BotToken string

	CatalogRepoOwner        string
	CatalogRepoName         string
	CatalogForkOrganization string
}

func (h *Handler) parseRequest(r *http.Request) (*api.Request, error) {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	webhookRequest := &api.Request{}
	if err := dec.Decode(webhookRequest); err != nil {
		return nil, fmt.Errorf("decode request body: %v", err)
	}

	if err := webhookRequest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %v", err)
	}
	return webhookRequest, nil
}

func (h *Handler) authenticateToken(ctx context.Context, r *api.Request) error {
	ghClient := gh.NewClient(ctx, r.GithubToken)
	appRepos, err := ghClient.AppRepos(ctx)
	if err != nil {
		return fmt.Errorf("get repo: %v", err)
	}
	if len(appRepos) != 1 {
		return fmt.Errorf("invalid github token: token must have permissions for exactly one repository")
	}
	if *appRepos[0].Owner.Login != r.PackageRepoOwner || *appRepos[0].Name != r.PackageRepoName {
		return fmt.Errorf("invalid github token: token does not have permissions on requested repository \"%s/%s\"", r.PackageRepoOwner, r.PackageRepoName)
	}
	return nil
}

func (h *Handler) forkCatalogRepo(ctx context.Context) (*github.Repository, error) {
	botClient := gh.NewClient(ctx, h.BotToken)
	log.Printf("forking \"%s/%s\" into organization %q", h.CatalogRepoOwner, h.CatalogRepoName, h.CatalogForkOrganization)
	fork, _, err := botClient.Repositories.CreateFork(ctx, h.CatalogRepoOwner, h.CatalogRepoName, &github.RepositoryCreateForkOptions{Organization: h.CatalogForkOrganization})
	if err != nil {
		aerr := &github.AcceptedError{}
		if errors.As(err, &aerr) {
			return fork, nil
		}
		return nil, fmt.Errorf("fork catalog repo: %v", err)
	}
	return fork, nil
}

func (h *Handler) cloneRepo(ctx context.Context, r *api.Request, repoURL string) (*git.Repository, error) {
	log.Printf("cloning fork %q", repoURL)
	return git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           repoURL,
		SingleBranch:  true,
		ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", r.CatalogBranch)),
		Auth: &githttp.BasicAuth{
			Username: "catalog-release-bot",
			Password: h.BotToken,
		},
	})
}

func (h *Handler) createBranch(repo *git.Repository, r *api.Request, now time.Time) (*git.Worktree, string, error) {
	wt, err := repo.Worktree()
	if err != nil {
		return nil, "", err
	}
	dateTime := now.Format("20060102-150405")
	branchName := fmt.Sprintf("%s-%s-%s-%s", r.PackageRepoOwner, r.PackageRepoName, r.PackageName, dateTime)
	log.Printf("creating branch %q", branchName)
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(branchName),
		Create: true,
		Force:  true,
		Keep:   false,
	}); err != nil {
		return nil, "", err
	}
	return wt, branchName, nil
}

func (h *Handler) commitPackageData(wt *git.Worktree, r *api.Request, now time.Time) error {
	catalogPkgDir := fmt.Sprintf("configs/%s", r.PackageName)

	log.Printf("removing existing package %q from catalog worktree", catalogPkgDir)
	if err := wt.Filesystem.Remove(catalogPkgDir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove existing package data from catalog worktree: %v", err)
	}

	log.Printf("create empty package directory %q in catalog worktree", catalogPkgDir)
	if err := wt.Filesystem.MkdirAll(catalogPkgDir, 0755); err != nil {
		return fmt.Errorf("create empty package directory in catalog worktree: %v", err)
	}
	for filename, data := range r.PackageData {
		filepath := fmt.Sprintf("%s/%s", catalogPkgDir, filename)
		f, err := wt.Filesystem.Create(filepath)
		if err != nil {
			return fmt.Errorf("create file %q in catalog worktree: %v", filepath, err)
		}
		defer f.Close()
		log.Printf("writing file %s", filename)
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write to file %q in catalog worktree: %v", filepath, err)
		}
	}

	log.Printf("adding directory %q to staging area", catalogPkgDir)
	if _, err := wt.Add(catalogPkgDir); err != nil {
		return fmt.Errorf("stage package data changes in catalog worktree: %v", err)
	}

	log.Printf("commiting changes")
	if _, err := wt.Commit(commitMessageFor(r), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "catalog-release-bot",
			Email: "catalog-release-bot@example.com",
			When:  now,
		},
	}); err != nil {
		return fmt.Errorf("commit package data changes in catalog worktree: %v", err)
	}
	return nil
}

func commitMessageFor(r *api.Request) string {
	return fmt.Sprintf("%s\n\n%s", titleFor(r), bodyFor(r))
}

func titleFor(r *api.Request) string {
	return fmt.Sprintf("%s: catalog-release-bot automatic update", r.PackageName)
}

func bodyFor(r *api.Request) string {
	return fmt.Sprintf(`This commit updates the catalog to reflect the desired package content from https://github.com/%s/%s/tree/%s`, r.PackageRepoOwner, r.PackageRepoName, r.PackageCommitHash)
}

func (h *Handler) pushBranch(ctx context.Context, repo *git.Repository, branchName string) error {
	remote, _ := repo.Remote("origin")
	head, _ := repo.Head()
	log.Printf("pushing branch %q to remote %q for repo %q", branchName, "origin", remote.Config().URLs[0])
	refSpec := fmt.Sprintf("%s:refs/heads/%s", head.Name(), branchName)
	return repo.PushContext(ctx, &git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{config.RefSpec(refSpec)},
		Auth: &githttp.BasicAuth{
			Username: "catalog-release-bot",
			Password: h.BotToken,
		},
		Force: true,
	})
}

func (h *Handler) createPullRequest(ctx context.Context, r *api.Request, branchName string) (*github.PullRequest, error) {
	log.Printf("creating pull request to pull branch %q into branch %q in repo \"%s/%s\"", branchName, r.CatalogBranch, h.CatalogRepoOwner, h.CatalogRepoName)
	cl := gh.NewClient(ctx, h.BotToken)
	prConfig := &github.NewPullRequest{
		Title:               github.String(titleFor(r)),
		Body:                github.String(bodyFor(r)),
		Head:                github.String(fmt.Sprintf("%s:%s", "catalog-release-bot", branchName)),
		Base:                github.String(r.CatalogBranch),
		MaintainerCanModify: github.Bool(false),
		Draft:               github.Bool(false),
	}
	pr, _, err := cl.PullRequests.Create(ctx, h.CatalogRepoOwner, h.CatalogRepoName, prConfig)
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Steps:
	//  1. Authenticate token and verify that it has package repo permissions
	//  2. Fork catalog repo
	//  3. Clone catalog repo locally
	//  4. Create branch from catalog base branch
	//  5. Write package data into branch and make commit
	//  8. Push branch to fork
	//  9. Create Pull Request to catalog repo

	ctx := r.Context()
	now := time.Now().UTC()
	webhookRequest, err := h.parseRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.authenticateToken(ctx, webhookRequest); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fork, err := h.forkCatalogRepo(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	repo, err := h.cloneRepo(ctx, webhookRequest, *fork.CloneURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("clone catalog repo: %v", err), http.StatusBadRequest)
		return
	}

	wt, branchName, err := h.createBranch(repo, webhookRequest, now)
	if err != nil {
		http.Error(w, fmt.Sprintf("create branch: %v", err), http.StatusInternalServerError)
		return
	}

	if err := h.commitPackageData(wt, webhookRequest, now); err != nil {
		http.Error(w, fmt.Sprintf("commit package data update: %v", err), http.StatusInternalServerError)
		return
	}
	if err := h.pushBranch(ctx, repo, branchName); err != nil {
		http.Error(w, fmt.Sprintf("push branch to fork: %v", err), http.StatusInternalServerError)
		return
	}

	if _, err := h.createPullRequest(ctx, webhookRequest, branchName); err != nil {
		http.Error(w, fmt.Sprintf("create pull request: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
