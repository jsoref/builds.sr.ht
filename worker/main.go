package main

import (
	"database/sql"
	"fmt"
	"os"
	"os/signal"

	_ "github.com/lib/pq"
	ms "github.com/mitchellh/mapstructure"
	celery "github.com/shicky/gocelery"
	ini "github.com/vaughan0/go-ini"
)

type WorkerContext struct {
	Db *sql.DB
}

func (ctx *WorkerContext) run_build(
	job_id int, _manifest map[string]interface{}) {

	job, err := GetJob(ctx.Db, job_id)
	if err != nil {
		fmt.Println(err)
		return
	}
	if err := job.SetStatus("running"); err != nil {
		fmt.Println(err)
		return
	}

	var manifest Manifest
	ms.Decode(_manifest, &manifest)
	fmt.Println(job, manifest)
}

func main() {
	var (
		config ini.File = nil
		err    error
	)
	for _, path := range []string{"../config.ini", "/etc/sr.ht/config.ini"} {
		config, err = ini.LoadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(err)
	}

	pgcs := conf(config, "builds.sr.ht", "connection-string")
	db, err := sql.Open("postgres", pgcs)
	if err != nil {
		panic(err)
	}

	redis := conf(config, "builds.sr.ht", "redis")

	broker := celery.NewRedisCeleryBroker(redis)
	backend := celery.NewRedisCeleryBackend(redis)

	client, err := celery.NewCeleryClient(broker, backend, 4)
	if err != nil {
		panic(err)
	}

	ctx := &WorkerContext{db}
	client.Register("buildsrht.runner.run_build", ctx.run_build)

	fmt.Println("Starting worker...")
	client.StartWorker()
	fmt.Println("Waiting for tasks.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	fmt.Println("Cleaning up...")

	client.StopWorker()
}

func conf(config ini.File, section string, key string) string {
	value, ok := config.Get(section, key)
	if !ok {
		panic(fmt.Errorf("Expected config option [%s]%s", section, key))
	}
	return value
}
