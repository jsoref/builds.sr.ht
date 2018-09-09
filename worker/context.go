package main

import (
	"context"
	"database/sql"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/go-redis/redis"
	"github.com/pkg/errors"

	ms "github.com/mitchellh/mapstructure"
)

type WorkerContext struct {
	Db    *sql.DB
	Redis *redis.Client
}

type JobContext struct {
	Cancel   context.CancelFunc
	Context  context.Context
	Db       *sql.DB
	Job      *Job
	LogDir   string
	LogFile  *os.File
	Log      *log.Logger
	Manifest *Manifest
	Port     int
}

func (wctx *WorkerContext) RunBuild(
	job_id int, _manifest map[string]interface{}) {

	var (
		job *Job
		ctx *JobContext
	)

	if !debug {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("run_build panic: %v", err)
				if job != nil {
					if ctx != nil &&
						ctx.Context.Err() == context.DeadlineExceeded {

						job.SetStatus("timeout")
					} else {
						job.SetStatus("failed")
					}
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

	goctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)

	ctx = &JobContext{
		Cancel:   cancel,
		Context:  goctx,
		Db:       wctx.Db,
		Job:      job,
		Manifest: &manifest,
	}

	jobsMutex.Lock()
	jobs[job_id] = ctx
	jobsMutex.Unlock()

	ctx.LogDir = path.Join(
		conf("builds.sr.ht::worker", "buildlogs"), strconv.Itoa(job_id))
	if err := os.MkdirAll(ctx.LogDir, 0755); err != nil {
		panic(errors.Wrap(err, "Make log directory"))
	}
	if ctx.LogFile, err = os.Create(path.Join(ctx.LogDir, "log")); err != nil {
		panic(errors.Wrap(err, "Make top-level log"))
	}
	defer ctx.LogFile.Close()

	ctx.Log = log.New(io.MultiWriter(ctx.LogFile, os.Stdout),
		"[#"+strconv.Itoa(job.Id)+"] ", log.LstdFlags)

	cleanup := ctx.Boot(wctx.Redis)
	defer cleanup()

	tasks := []func() error{
		ctx.Settle,
		ctx.SendTasks,
		ctx.SendEnv,
		ctx.SendSecrets,
		ctx.ConfigureRepos,
		ctx.CloneRepos,
		ctx.InstallPackages,
		ctx.RunTasks,
	}
	for _, task := range tasks {
		if err := task(); err != nil {
			panic(err)
		}
	}

	jobsMutex.Lock()
	delete(jobs, job_id)
	jobsMutex.Unlock()

	cancel()
	job.SetStatus("success")
}

func (ctx *JobContext) Control(
	context context.Context, args ...string) *exec.Cmd {

	control := conf("builds.sr.ht::worker", "controlcmd")
	return exec.CommandContext(context, control, args...)
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

func (ctx *JobContext) Tee(path string, data []byte) error {
	tee := ctx.SSH("tee", path)
	pipe, err := tee.StdinPipe()
	if err != nil {
		return err
	}
	if err := tee.Start(); err != nil {
		return err
	}
	if _, err := pipe.Write(data); err != nil {
		return err
	}
	pipe.Close()
	return tee.Wait()
}
