package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"

	goredis "github.com/go-redis/redis/v8"
	"github.com/google/shlex"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ms "github.com/mitchellh/mapstructure"
)

var (
	buildsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_builds_started_total",
		Help: "The total number of builds which have been started",
	})
	buildsFinished = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "buildsrht_builds_finished_total",
		Help: "The total number of finished builds by status",
	}, []string{"status"})
	buildDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "buildsrht_build_duration_seconds",
		Help:    "Duration of each build in seconds",
		Buckets: []float64{10, 30, 60, 90, 120, 300, 600, 900, 1800},
	})
)

type WorkerContext struct {
	Db    *sql.DB
	Redis *goredis.Client
	Conf  func(section, key string) string
}

type JobContext struct {
	Cancel      context.CancelFunc
	Claimed     bool
	Conf        func(section, key string) string
	Context     context.Context
	Db          *sql.DB
	Deadline    time.Time
	ImageConfig *ImageConfig
	Job         *Job
	Log         *log.Logger
	LogDir      string
	LogFile     *os.File
	Manifest    *Manifest
	Port        int
	Settled     bool

	NTasks int
	Task   int

	ProcessedTriggers bool
}

func (wctx *WorkerContext) RunBuild(
	job_id int, _manifest map[string]interface{}) error {

	var (
		err error
		job *Job
		ctx *JobContext

		cleanup func()
	)

	buildUser, ok := config.Get("git.sr.ht::dispatch", "/usr/bin/buildsrht-keys")
	if !ok {
		buildUser = "builds:builds"
	}
	buildUser = strings.Split(buildUser, ":")[0]

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
				if ctx.Context.Err() == context.DeadlineExceeded {
					buildsFinished.WithLabelValues("timeout").Inc()
					job.SetStatus("timeout")
				} else if ctx.Context.Err() == context.Canceled {
					buildsFinished.WithLabelValues("cancelled").Inc()
					job.SetStatus("cancelled")
				} else {
					buildsFinished.WithLabelValues("failed").Inc()
					job.SetStatus("failed")
				}
				ctx.ProcessTriggers()
				if ctx.Settled {
					ctx.Standby(buildUser)
				}
				if ctx.Log != nil {
					ctx.Log.Printf("Error: %v\n", err)
					ctx.LogFile.Close()
				}
			} else if job != nil {
				buildsFinished.WithLabelValues("failed").Inc()
				job.SetStatus("failed")
			} else {
				buildsFinished.WithLabelValues("failed").Inc()
			}
		}
		if cleanup != nil {
			cleanup()
		}
	}()

	timeout, _ := time.ParseDuration(conf("builds.sr.ht::worker", "timeout"))
	goctx, cancel := context.WithTimeout(context.Background(), timeout)

	ctx = &JobContext{
		Cancel:   cancel,
		Conf:     wctx.Conf,
		Context:  goctx,
		Db:       wctx.Db,
		Deadline: time.Now().UTC().Add(timeout),
		Job:      job,
		Manifest: &manifest,
	}

	jobsMutex.Lock()
	jobs[job_id] = ctx
	jobsMutex.Unlock()

	ctx.ImageConfig = LoadImageConfig(manifest.Image)

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

	cleanup = ctx.Boot(wctx.Redis)

	if err = ctx.Settle(); err != nil {
		panic(err)
	}

	if manifest.Shell {
		ctx.Log.Println()
		ctx.Log.Println("\x1B[1m\x1B[96mShell access for this build was requested.\x1B[0m")
		ctx.Log.Println("To log in with SSH, use the following command:")
		ctx.Log.Println()
		ctx.Log.Printf("\tssh -t %s@%s connect %d", buildUser, runner, job_id)
		ctx.Log.Println()
	}

	tasks := []func() error{
		ctx.SendTasks,
		ctx.SendEnv,
		ctx.SendSecrets,
		ctx.SendHutConfig,
		ctx.ConfigureRepos,
		ctx.InstallPackages,
		ctx.CloneRepos,
		ctx.RunTasks,
		ctx.UploadArtifacts,
	}
	ctx.NTasks = len(tasks)
	for i, task := range tasks {
		ctx.Task = i
		if err = task(); err != nil {
			panic(err)
		}
	}
	ctx.Task = ctx.NTasks

	if manifest.Shell {
		<-goctx.Done()
	}

	jobsMutex.Lock()
	delete(jobs, job_id)
	jobsMutex.Unlock()

	cancel()
	job.SetStatus("success")
	ctx.ProcessTriggers()
	ctx.LogFile.Close()

	buildsFinished.WithLabelValues("success").Inc()
	return nil
}

func (ctx *JobContext) Standby(buildUser string) {
	ctx.Log.Println("\x1B[1m\x1B[91mBuild failed.\x1B[0m")
	ctx.Log.Println("The build environment will be kept alive for 10 minutes.")
	ctx.Log.Println("To log in with SSH and examine it, use the following command:")
	ctx.Log.Println()
	ctx.Log.Printf("\tssh -t %s@%s connect %d", buildUser, *ctx.Job.Runner, ctx.Job.Id)
	ctx.Log.Println()
	ctx.Log.Println("After logging in, the deadline is increased to your remaining build time.")
	select {
	case <-time.After(10 * time.Minute):
		break
	case <-ctx.Context.Done():
		ctx.Log.Println("Build cancelled. Terminating build environment.")
		return
	}
	if ctx.Claimed {
		select {
		case <-time.After(time.Until(ctx.Deadline)):
			break
		case <-ctx.Context.Done():
			break
		}
	} else {
		ctx.Log.Println("Deadline elapsed. Terminating build environment.")
	}
}

func (ctx *JobContext) Control(
	context context.Context, args ...string) *exec.Cmd {

	controlString := conf("builds.sr.ht::worker", "controlcmd")
	controlSplitted, err := shlex.Split(controlString)
	if err != nil {
		panic(errors.Wrap(err, "controlcmd"))
	}
	args = append(controlSplitted[1:], args...)

	return exec.CommandContext(context, controlSplitted[0], args...)
}

func (ctx *JobContext) SSH(args ...string) *exec.Cmd {
	sport := strconv.Itoa(ctx.Port)
	switch ctx.ImageConfig.LoginCmd {
		case "drawterm":
			return exec.CommandContext(ctx.Context,
				"env", fmt.Sprintf("DIALSRV=%s", sport),
				"PASS=password", "drawterm",
				"-a", "none",
				"-u", "glenda",
				"-h", "127.0.0.1",
				"-Gc", strings.Join(args, " "))
		case "ssh":
			return exec.CommandContext(ctx.Context, "ssh",
				append([]string{"-q",
					"-p", sport,
					"-o", "UserKnownHostsFile=/dev/null",
					"-o", "StrictHostKeyChecking=no",
					"-o", "LogLevel=quiet",
					"build@localhost",
				}, args...)...)
		default:
			panic(errors.New("Unknown login command"))
	}
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

func (ctx *JobContext) FileSize(path string) (int64, error) {
	wc := ctx.SSH("wc", "-c", path)
	pipe, err := wc.StdoutPipe()
	defer pipe.Close()
	if err != nil {
		return 0, err
	}
	if err := wc.Start(); err != nil {
		return 0, err
	}
	stdout, err := ioutil.ReadAll(io.LimitReader(pipe, 1024))
	if err != nil {
		return 0, err
	}
	if len(stdout) == 0 {
		return 0, errors.New("File not found")
	}
	parts := strings.Split(strings.Trim(string(stdout), " \t"), " ")
	if len(parts) != 2 {
		return 0, errors.New("Unexpected response from wc")
	}
	return strconv.ParseInt(parts[0], 10, 64)
}

func (ctx *JobContext) Download(path string) (io.ReadCloser, error) {
	cat := ctx.SSH("cat", path)
	pipe, err := cat.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cat.Start(); err != nil {
		return nil, err
	}
	return pipe, nil
}
