package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"git.sr.ht/~sircmpwn/core-go/crypto"
	goredis "github.com/go-redis/redis/v8"
	"github.com/vaughan0/go-ini"

	celery "github.com/gocelery/gocelery"
	_ "github.com/lib/pq"
)

var (
	config  ini.File
	origin  string
	workers int

	jobs      map[int]*JobContext
	jobsMutex sync.Mutex
)

func main() {
	var configPath string
	flag.IntVar(&workers, "workers", runtime.NumCPU(),
		"configure number of workers")
	flag.StringVar(&configPath, "config", "../config.ini",
		"path to config.ini file")
	flag.Parse()

	var err error
	for _, path := range []string{configPath, "/etc/sr.ht/config.ini"} {
		config, err = ini.LoadFile(path)
		if err == nil {
			break
		}
	}
	if err != nil {
		panic(err)
	}
	crypto.InitCrypto(config)

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
		redisHost = "redis://localhost:6379"
	}
	ropts, err := goredis.ParseURL(redisHost)
	if err != nil {
		panic(err)
	}
	localRedis := goredis.NewClient(ropts)
	if _, err := localRedis.Ping(context.Background()).Result(); err != nil {
		panic(err)
	}

	ctx := &WorkerContext{db, localRedis, conf}
	client.Register("buildsrht.runner.run_build", ctx.RunBuild)

	log.Printf("Starting %d workers...", workers)
	go HttpServer()
	client.StartWorker()
	log.Println("Waiting for tasks.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	log.Println("Waiting for workers to terminate...")
	client.StopWorker()
}

func conf(section string, key string) string {
	value, ok := config.Get(section, key)
	if !ok {
		log.Fatalf("Expected config option [%s]%s", section, key)
	}
	return value
}
