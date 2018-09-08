package main

import (
	"database/sql"
	"log"
	"os/exec"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"

	ms "github.com/mitchellh/mapstructure"
)

type WorkerContext struct {
	Db *sql.DB
	Redis *redis.Client
}

type JobContext struct {
	Db       *sql.DB
	Job      *Job
	Manifest *Manifest
	Port     int
}

func (wctx *WorkerContext) RunBuild(
	job_id int, _manifest map[string]interface{}) {

	defer func() {
		if err := recover(); err != nil {
			log.Printf("run_build panic: %v", err)
		}
	}()

	var manifest Manifest
	ms.Decode(_manifest, &manifest)

	job, err := GetJob(wctx.Db, job_id)
	if err != nil {
		panic(errors.Wrap(err, "GetJob"))
	}
	if err := job.SetStatus("running"); err != nil {
		panic(errors.Wrap(err, "job.SetStatus"))
	}

	ctx := &JobContext{
		Db: wctx.Db,
		Job: job,
		Manifest: &manifest,
	}

	cleanup := ctx.Boot(wctx.Redis)
	defer cleanup()

	time.Sleep(10 * time.Second)
}

func (ctx *JobContext) Control(args ...string) *exec.Cmd {
	control := conf("builds.sr.ht::worker", "controlcmd")
	return exec.Command(control, args...)
}

func (ctx *JobContext) SSH(args ...string) *exec.Cmd {
	return nil
}
