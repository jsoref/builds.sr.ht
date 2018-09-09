package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"time"

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

func (ctx *JobContext) SanityCheck() error {
	log.Println("Running sanity check")
	timeout, _ := context.WithTimeout(ctx.Context, 60 * time.Second)
	done := make(chan error, 1)
	attempt := 0
	go func() {
		for {
			attempt++
			check := ctx.SSH("echo", "hello world")
			pipe, _ := check.StdoutPipe()
			if err := check.Start(); err != nil {
				done <-err
				return
			}
			stdout, _ := ioutil.ReadAll(pipe)
			if err := check.Wait(); err == nil {
				if string(stdout) == "hello world\n" {
					log.Printf("Sanity check passed.")
					done <-nil
					return
				} else {
					done <-fmt.Errorf("Unexpected sanity check output: %s",
						string(stdout))
					return
				}
			}

			select {
			case <-timeout.Done():
				done <-fmt.Errorf("Sanity check timed out after %d attempts",
					attempt)
				return
			case <-time.After(1 * time.Second):
				// Loop
			}
		}
	}()
	return <-done
}
