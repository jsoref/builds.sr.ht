package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/go-redis/redis"
	"github.com/vaughan0/go-ini"

	_ "github.com/lib/pq"
	celery "github.com/shicky/gocelery"
)

var (
	config ini.File
	debug  bool
)

func main() {
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()

	var err error
	for _, path := range []string{"../config.ini", "/etc/sr.ht/config.ini"} {
		config, err = ini.LoadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(err)
	}

	pgcs := conf("builds.sr.ht", "connection-string")
	db, err := sql.Open("postgres", pgcs)
	if err != nil {
		panic(err)
	}

	clusterRedis := conf("builds.sr.ht", "redis")
	broker := celery.NewRedisCeleryBroker(clusterRedis)
	backend := celery.NewRedisCeleryBackend(clusterRedis)

	client, err := celery.NewCeleryClient(broker, backend, 4)
	if err != nil {
		panic(err)
	}

	localRedis := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if _, err := localRedis.Ping().Result(); err != nil {
		panic(err)
	}

	ctx := &WorkerContext{db, localRedis}
	client.Register("buildsrht.runner.run_build", ctx.RunBuild)

	log.Println("Starting worker...")
	client.StartWorker()
	log.Println("Waiting for tasks.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	log.Println("Cleaning up...")

	client.StopWorker()
}

func conf(section string, key string) string {
	value, ok := config.Get(section, key)
	if !ok {
		log.Fatalf("Expected config option [%s]%s", section, key)
	}
	return value
}
