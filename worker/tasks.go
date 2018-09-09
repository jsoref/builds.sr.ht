package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
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
	log.Println("Waiting for guest to settle")
	timeout, _ := context.WithTimeout(ctx.Context, 60*time.Second)
	done := make(chan error, 1)
	attempt := 0
	go func() {
		for {
			attempt++
			check := ctx.SSH("echo", "hello world")
			pipe, _ := check.StdoutPipe()
			if err := check.Start(); err != nil {
				done <- err
				return
			}
			stdout, _ := ioutil.ReadAll(pipe)
			if err := check.Wait(); err == nil {
				if string(stdout) == "hello world\n" {
					done <- nil
					return
				} else {
					done <- fmt.Errorf("Unexpected sanity check output: %s",
						string(stdout))
					return
				}
			}

			select {
			case <-timeout.Done():
				done <- fmt.Errorf("Sanity check timed out after %d attempts",
					attempt)
				return
			case <-time.After(1 * time.Second):
				// Loop
			}
		}
	}()
	return <-done
}

const preamble = `#!/usr/bin/env bash
. ~/.buildenv
set -xe
`

func (ctx *JobContext) SendTasks() error {
	log.Println("Sending tasks")
	const home = "/home/build"
	taskdir := path.Join(home, ".tasks")
	if err := ctx.SSH("mkdir", "-p", taskdir).Run(); err != nil {
		return err
	}
	for _, task := range ctx.Manifest.Tasks {
		var name, script string
		for name, script = range task {
			break
		}
		taskpath := path.Join(taskdir, name)
		cmd := ctx.SSH("tee", taskpath)
		pipe, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		if _, err := pipe.Write([]byte(preamble + script)); err != nil {
			return err
		}
		pipe.Close()
		if err := cmd.Wait(); err != nil {
			return err
		}
		if err := ctx.SSH("chmod", "755", taskpath).Run(); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *JobContext) RunTasks() error {
	for _, task := range ctx.Manifest.Tasks {
		var (
			err   error
			logfd *os.File
			name  string
			ssh   *exec.Cmd
		)
		for name, _ = range task {
			break
		}

		log.Printf("Running task %s\n", name)
		ctx.Job.SetTaskStatus(name, "running")

		if err = os.Mkdir(path.Join(ctx.LogDir, name), 0755); err != nil {
			goto fail
		}

		ssh = ctx.SSH(path.Join(".", ".tasks", name))
		if logfd, err = os.Create(path.Join(ctx.LogDir, name, "log"));
			err != nil {

			err = errors.Wrap(err, "Creating log file")
			goto fail
		}
		ssh.Stdout = logfd
		ssh.Stderr = logfd

		if err = ssh.Run(); err != nil {
			exiterr, ok := err.(*exec.ExitError)
			if !ok {
				goto fail
			}
			status, ok := exiterr.Sys().(unix.WaitStatus)
			if !ok {
				goto fail
			}
			if status.ExitStatus() == 255 {
				log.Println("TODO: Mark remaining tasks as skipped")
				ctx.Job.SetTaskStatus(name, "success")
				break
			}
			err = errors.Wrap(err, "Running task on guest")
			goto fail
		}

		ctx.Job.SetTaskStatus(name, "success")
		continue
fail:
		ctx.Job.SetTaskStatus(name, "failed")
		return err
	}
	return nil
}
