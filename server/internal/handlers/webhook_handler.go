package handlers

import (
	"net/http"

	"github.com/Mentro-Org/CodeLookout/internal/config"
	"github.com/Mentro-Org/CodeLookout/internal/core"
	ghclient "github.com/Mentro-Org/CodeLookout/internal/github"
	pr "github.com/Mentro-Org/CodeLookout/internal/handlers/pullrequest"
	"github.com/Mentro-Org/CodeLookout/internal/llm"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/go-github/v72/github"
)

type WebhookHandlerService struct {
	Cfg             *config.Config
	GHClientFactory *ghclient.ClientFactory
	AIClient        llm.AIClient
	DBPool          *pgxpool.Pool
}

func NewWebhookHandlerService(cfg *config.Config, ghClientFactory *ghclient.ClientFactory, aiClient llm.AIClient, dbPool *pgxpool.Pool) WebhookHandlerService {
	return WebhookHandlerService{
		Cfg:             cfg,
		GHClientFactory: ghClientFactory,
		AIClient:        aiClient,
		DBPool:          dbPool,
	}
}

func (s *WebhookHandlerService) HandleWebhook() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		payload, err := github.ValidatePayload(r, []byte(s.Cfg.WebhookSecret))
		if err != nil {
			http.Error(w, "Invalid payload signature", http.StatusUnauthorized)
			return
		}

		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			http.Error(w, "Could not parse webhook", http.StatusBadRequest)
			return
		}

		switch e := event.(type) {
		case *github.PullRequestEvent:
			handler := s.routePullRequestEvent(e.GetAction())
			if handler == nil {
				http.Error(w, "Unsupported PR action", http.StatusNotImplemented)
				return
			}
			if err := handler.Handle(ctx, e); err != nil {
				http.Error(w, "Handler error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "Unsupported event type", http.StatusNotImplemented)
			return

		}

		w.WriteHeader(http.StatusOK)
	}
}

func (s *WebhookHandlerService) routePullRequestEvent(action string) core.PullRequestHandler {
	switch action {
	case "opened":
		return &pr.PullRequestOpenedHandler{Cfg: s.Cfg, GHClientFactory: s.GHClientFactory, AIClient: s.AIClient, DBPool: s.DBPool}
	case "edited":
		return &pr.PullRequestEditedHandler{Cfg: s.Cfg, GHClientFactory: s.GHClientFactory, AIClient: s.AIClient, DBPool: s.DBPool}
	case "synchronize":
		return &pr.PullRequestSynchronizedHandler{Cfg: s.Cfg, GHClientFactory: s.GHClientFactory, AIClient: s.AIClient, DBPool: s.DBPool}
	default:
		return nil
	}
}
