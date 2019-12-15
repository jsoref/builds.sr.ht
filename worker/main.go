package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/go-redis/redis"
	"github.com/vaughan0/go-ini"

	_ "github.com/lib/pq"
	celery "github.com/shicky/gocelery"
)

var (
	config  ini.File
	origin  string
	workers int

	jobs      map[int]*JobContext
	jobsMutex sync.Mutex
)

func main() {
	flag.IntVar(&workers, "workers", runtime.NumCPU(),
		"configure number of workers")
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

	jobs = make(map[int]*JobContext, 1)

	pgcs := conf("builds.sr.ht", "connection-string")
	db, err := sql.Open("postgres", pgcs)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to open a database connection: %v", err)
	}

	origin = conf("builds.sr.ht", "origin")

	clusterRedis := conf("builds.sr.ht", "redis")
	broker := celery.NewRedisCeleryBroker(clusterRedis)
	backend := celery.NewRedisCeleryBackend(clusterRedis)

	client, err := celery.NewCeleryClient(broker, backend, workers)
	if err != nil {
		panic(err)
	}
	redisHost, ok := config.Get("sr.ht", "redis-host")
	if !ok {
		redisHost = "localhost:6379"
	}
	localRedis := redis.NewClient(&redis.Options{Addr: redisHost})
	if _, err := localRedis.Ping().Result(); err != nil {
		panic(err)
	}

	ctx := &WorkerContext{db, localRedis, conf}
	client.Register("buildsrht.runner.run_build", ctx.RunBuild)

	log.Println("Starting worker...")
	go HttpServer()
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
