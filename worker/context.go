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
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ms "github.com/mitchellh/mapstructure"
)

var (
	buildsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_builds_started",
		Help: "The total number of builds which have been started",
	})
	successfulBuilds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_build_successes",
		Help: "The total number of builds which completed successfully",
	})
	failedBuilds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_build_failed",
		Help: "The total number of builds which failed",
	})
	timeoutBuilds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_build_timed_out",
		Help: "The total number of builds which timed out",
	})
	cancelledBuilds = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_build_cancelled",
		Help: "The total number of builds which were cancelled",
	})
	buildDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "buildsrht_build_duration",
		Help: "Duration of each build",

		Buckets: []float64{10, 30, 60, 90, 120, 300, 600, 900, 1800},
	})
)

type WorkerContext struct {
	Db    *sql.DB
	Redis *redis.Client
	Conf  func(section, key string) string
}

type JobContext struct {
	Cancel   context.CancelFunc
	Conf     func(section, key string) string
	Context  context.Context
	Db       *sql.DB
	Job      *Job
	LogDir   string
	LogFile  *os.File
	Log      *log.Logger
	Manifest *Manifest
	Port     int

	ProcessedTriggers bool
}

func (wctx *WorkerContext) RunBuild(
	job_id int, _manifest map[string]interface{}) {

	var (
		err error
		job *Job
		ctx *JobContext
	)

	timer := prometheus.NewTimer(buildDuration)
	defer timer.ObserveDuration()
	buildsStarted.Inc()

	var manifest Manifest
	ms.Decode(_manifest, &manifest)

	job, err = GetJob(wctx.Db, job_id)
	if err != nil {
		panic(errors.Wrap(err, "GetJob"))
	}
	runner := conf("builds.sr.ht::worker", "name")
	if err = job.SetRunner(runner); err != nil {
		panic(errors.Wrap(err, "job.SetRunner"))
	}
	if err = job.SetStatus("running"); err != nil {
		panic(errors.Wrap(err, "job.SetStatus"))
	}

	if !job.Secrets {
		manifest.Secrets = []string{}
	}

	defer func() {
		if err := recover(); err != nil {
			log.Printf("run_build panic: %v", err)
			if job != nil && ctx != nil {
				failedBuilds.Inc()
				if ctx.Context.Err() == context.DeadlineExceeded {
					timeoutBuilds.Inc()
					job.SetStatus("timeout")
				} else if ctx.Context.Err() == context.Canceled {
					cancelledBuilds.Inc()
					job.SetStatus("cancelled")
				} else {
					job.SetStatus("failed")
				}
				ctx.ProcessTriggers()
				if ctx.Log != nil {
					ctx.Log.Printf("Error: %v\n", err)
					ctx.LogFile.Close()
				}
			} else if job != nil {
				job.SetStatus("failed")
			}
			failedBuilds.Inc()
		}
	}()

	timeout, _ := time.ParseDuration(conf("builds.sr.ht::worker", "timeout"))
	goctx, cancel := context.WithTimeout(context.Background(), timeout)

	ctx = &JobContext{
		Cancel:   cancel,
		Conf:     wctx.Conf,
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
	if err = os.MkdirAll(ctx.LogDir, 0755); err != nil {
		panic(errors.Wrap(err, "Make log directory"))
	}
	if ctx.LogFile, err = os.Create(path.Join(ctx.LogDir, "log")); err != nil {
		panic(errors.Wrap(err, "Make top-level log"))
	}

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
		ctx.InstallPackages,
		ctx.CloneRepos,
		ctx.RunTasks,
	}
	for _, task := range tasks {
		if err = task(); err != nil {
			panic(err)
		}
	}

	if manifest.Shell {
		ctx.Log.Println()
		ctx.Log.Println("\x1B[1m\x1B[96mShell access for this build was requested.\x1B[0m")
		ctx.Log.Println("To log in with SSH, use the following command:")
		ctx.Log.Println()
		ctx.Log.Printf("\tssh -t builds@%s connect %d", runner, job_id)
		ctx.Log.Println()
		<-goctx.Done()
	}

	jobsMutex.Lock()
	delete(jobs, job_id)
	jobsMutex.Unlock()

	cancel()
	job.SetStatus("success")
	ctx.ProcessTriggers()
	ctx.LogFile.Close()

	successfulBuilds.Inc()
}

func (ctx *JobContext) Control(
	context context.Context, args ...string) *exec.Cmd {

	control := conf("builds.sr.ht::worker", "controlcmd")
	return exec.CommandContext(context, control, args...)
}

func (ctx *JobContext) SSH(args ...string) *exec.Cmd {
	sport := strconv.Itoa(ctx.Port)
	return exec.CommandContext(ctx.Context, "ssh",
		append([]string{"-q", "-t",
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
	tee.Stderr = ctx.LogFile
	if err := tee.Start(); err != nil {
		return err
	}
	if _, err := pipe.Write(data); err != nil {
		return err
	}
	pipe.Close()
	return tee.Wait()
}
