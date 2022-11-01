package main

import (
	"context"
	"fmt"

	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/server"
	"git.sr.ht/~sircmpwn/core-go/webhooks"
	work "git.sr.ht/~sircmpwn/dowork"
	"github.com/99designs/gqlgen/graphql"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/account"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/loaders"
)

func main() {
	appConfig := config.LoadConfig(":5102")

	gqlConfig := api.Config{Resolvers: &graph.Resolver{}}
	gqlConfig.Directives.Private = server.Private
	gqlConfig.Directives.Internal = server.Internal
	gqlConfig.Directives.Access = func(ctx context.Context, obj interface{},
		next graphql.Resolver, scope model.AccessScope,
		kind model.AccessKind) (interface{}, error) {
		return server.Access(ctx, obj, next, scope.String(), kind.String())
	}
	gqlConfig.Directives.Worker = func(ctx context.Context, obj interface{},
		next graphql.Resolver) (interface{}, error) {
		return nil, fmt.Errorf("Access denied")
	}
	schema := api.NewExecutableSchema(gqlConfig)

	scopes := make([]string, len(model.AllAccessScope))
	for i, s := range model.AllAccessScope {
		scopes[i] = s.String()
	}

	accountQueue := work.NewQueue("account")
	webhookQueue := webhooks.NewQueue(schema)

	server.NewServer("builds.sr.ht", appConfig).
		WithDefaultMiddleware().
		WithMiddleware(
			loaders.Middleware,
			account.Middleware(accountQueue),
			webhooks.Middleware(webhookQueue),
		).
		WithSchema(schema, scopes).
		WithQueues(
			accountQueue,
			webhookQueue.Queue,
		).
		Run()
}
