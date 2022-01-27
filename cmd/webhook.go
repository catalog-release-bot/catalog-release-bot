package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	"github.com/catalog-release-bot/catalog-release-bot/pkg/webhook"
)

func newWebhookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "webhook",
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			botToken, ok := os.LookupEnv("BOT_TOKEN")
			if !ok {
				log.Fatal("must set environment variable BOT_TOKEN")
			}

			port, ok := os.LookupEnv("PORT")
			if !ok {
				port = "8080"
			}

			s := webhook.Server{
				Addr:            fmt.Sprintf(":%s", port),
				ShutdownTimeout: time.Second * 60,
			}

			handler := mux.NewRouter()
			handler.Handle("/webhook", handlers.ContentTypeHandler(&webhook.Handler{
				CatalogRepoOwner:        "joelanford",
				CatalogRepoName:         "example-catalog",
				CatalogForkOrganization: "catalog-release-bot",
				BotToken:                botToken,
			}, "application/json")).Methods(http.MethodPost)

			if err := s.Run(cmd.Context(), handler); err != nil {
				log.Fatal(err)
			}
		},
	}
	return cmd
}
