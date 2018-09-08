package main

import (
	"log"
	"os"
	"strconv"

	"github.com/go-redis/redis"
)

func (ctx *JobContext) Boot(r *redis.Client) func() {
	port, err := r.Incr("builds.sr.ht.ssh-port").Result()
	if err == nil && port < 22000 {
		port = 22000
		err = r.Set("builds.sr.ht.ssh-port", port, 0).Err()
	} else if err == nil && port >= 23000 {
		port = 22000
		err = r.Set("builds.sr.ht.ssh-port", port, 0).Err()
	}
	if err != nil {
		panic(err)
	}

	ctx.Port = int(port)
	log.Printf("Booting image %s on port %d", ctx.Manifest.Image, port)
	sport := strconv.Itoa(int(port))

	boot := ctx.Control(ctx.Manifest.Image, "boot", sport)
	boot.Stdout = os.Stdout
	boot.Stderr = os.Stderr
	if err := boot.Start(); err != nil {
		panic(err)
	}

	return func() {
		log.Printf("Cleaning up build on port %d", port)
		cleanup := ctx.Control(ctx.Manifest.Image, "cleanup", sport)
		cleanup.Run()
	}
}
