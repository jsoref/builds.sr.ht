package graph

import (
	"context"

	"git.sr.ht/~sircmpwn/core-go/config"

	celery "github.com/gocelery/gocelery"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	buildsSubmitted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_builds_submited_total",
		Help: "Number of builds submitted",
	})
)

func SubmitJob(ctx context.Context, jobID int, manifest *Manifest) error {
	conf := config.ForContext(ctx)
	clusterRedis, _ := conf.Get("builds.sr.ht", "redis")
	broker := celery.NewRedisCeleryBroker(clusterRedis)
	backend := celery.NewRedisCeleryBackend(clusterRedis)

	// XXX: Maybe we should keep this client instance around and stash it
	// somewhere on the context
	client, err := celery.NewCeleryClient(broker, backend, 1)
	if err != nil {
		panic(err)

	}

	buildsSubmitted.Inc()
	_, err = client.Delay("buildsrht.runner.run_build", jobID, manifest)
	return err
}
