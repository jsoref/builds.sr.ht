package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"

	ms "github.com/mitchellh/mapstructure"
)

type WorkerContext struct {
	Db    *sql.DB
	Redis *redis.Client
}

type JobContext struct {
	Context  context.Context
	Db       *sql.DB
	Job      *Job
	LogDir   string
	Manifest *Manifest
	Port     int
}

func (wctx *WorkerContext) RunBuild(
	job_id int, _manifest map[string]interface{}) {

	var job *Job = nil

	if !debug {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("run_build panic: %v", err)
				if job != nil {
					job.SetStatus("failed")
				}
			}
		}()
	}

	var manifest Manifest
	ms.Decode(_manifest, &manifest)

	job, err := GetJob(wctx.Db, job_id)
	if err != nil {
		panic(errors.Wrap(err, "GetJob"))
	}
	if err := job.SetRunner(conf("builds.sr.ht::worker", "name")); err != nil {
		panic(errors.Wrap(err, "job.SetRunner"))
	}
	if err := job.SetStatus("running"); err != nil {
		panic(errors.Wrap(err, "job.SetStatus"))
	}

	ctx := &JobContext{
		Context:  context.TODO(),
		Db:       wctx.Db,
		Job:      job,
		Manifest: &manifest,
	}

	cleanup := ctx.Boot(wctx.Redis)
	defer cleanup()

	ctx.LogDir = path.Join(
		conf("builds.sr.ht::worker", "buildlogs"), strconv.Itoa(job_id))
	if err := os.MkdirAll(ctx.LogDir, 0755); err != nil {
		panic(errors.Wrap(err, "Make log directory"))
	}

	tasks := []func() error{
		ctx.SanityCheck,
		ctx.SendTasks,
		ctx.SendEnv,
		// TODO: secrets
		// TODO: custom repos
		// TODO: git repos
		// TODO: packages
		ctx.RunTasks,
	}
	for _, task := range tasks {
		if err := task(); err != nil {
			panic(err)
		}
	}

	job.SetStatus("success")
}

func (ctx *JobContext) Control(args ...string) *exec.Cmd {
	control := conf("builds.sr.ht::worker", "controlcmd")
	return exec.Command(control, args...)
}

func (ctx *JobContext) SSH(args ...string) *exec.Cmd {
	sport := strconv.Itoa(ctx.Port)
	return exec.CommandContext(ctx.Context, "ssh",
		append([]string{"-q",
			"-p", sport,
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "StrictHostKeyChecking=no",
			"-o", "LogLevel=quiet",
			"build@localhost",
		}, args...)...)
}
