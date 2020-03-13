package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis"
	"github.com/kr/pty"
	"github.com/minio/minio-go/v6"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	buildsRun = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "buildsrht_image_runs_total",
		Help: "The total number of builds run per arch and image",
	}, []string{"image", "arch"})
	settleTime = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "buildsrht_vm_settle_duration_seconds",
		Help:    "Duration taken by a VM to settle in seconds",
		Buckets: []float64{1, 2, 3, 5, 10, 30, 60, 90, 120, 300},
	}, []string{"image", "arch"})
)

func (ctx *JobContext) Boot(r *redis.Client) func() {
	port, err := r.Incr("builds.sr.ht.ssh-port").Result()
	if err == nil && port < 22000 {
		err = r.Set("builds.sr.ht.ssh-port", 22100, 0).Err()
	} else if err == nil && port >= 23000 {
		err = r.Set("builds.sr.ht.ssh-port", 22000, 0).Err()
	}
	if err != nil {
		panic(errors.Wrap(err, "assign port"))
	}

	arch := "default"
	if ctx.Manifest.Arch != nil {
		arch = *ctx.Manifest.Arch
	}
	ctx.Port = int(port)
	ctx.Log.Printf("Booting image %s (%s) on port %d",
		ctx.Manifest.Image, arch, port)

	boot := ctx.Control(ctx.Context,
		ctx.Manifest.Image, "boot", arch, strconv.Itoa(ctx.Port))
	boot.Env = append(os.Environ(), fmt.Sprintf("BUILD_JOB_ID=%d", ctx.Job.Id))
	boot.Stdout = ctx.LogFile
	boot.Stderr = ctx.LogFile
	if err := boot.Run(); err != nil {
		panic(errors.Wrap(err, "boot"))
	}

	buildsRun.WithLabelValues(ctx.Manifest.Image, arch).Inc()

	return func() {
		ctx.Log.Printf("Tearing down build VM")
		cleanup := ctx.Control(context.TODO(), ctx.Manifest.Image, "cleanup",
			strconv.Itoa(ctx.Port))
		if err := cleanup.Run(); err != nil {
			fmt.Printf("Failed to destroy build VM: %v\n", err)
		}
	}
}

func (ctx *JobContext) Settle() error {
	ctx.Log.Println("Waiting for guest to settle")

	arch := "default"
	if ctx.Manifest.Arch != nil {
		arch = *ctx.Manifest.Arch
	}
	settleTimer := prometheus.NewTimer(settleTime.WithLabelValues(ctx.Manifest.Image, arch))
	defer settleTimer.ObserveDuration()

	timeout, cancel := context.WithTimeout(ctx.Context, 120*time.Second)
	defer cancel()
	done := make(chan error, 1)
	attempt := 0
	go func() {
		for {
			attempt++
			check := ctx.SSH("printf", "'hello world'")
			pipe, _ := check.StdoutPipe()
			if err := check.Start(); err != nil {
				done <- err
				return
			}
			stdout, _ := ioutil.ReadAll(pipe)
			if err := check.Wait(); err == nil {
				if string(stdout) == "hello world" {
					ctx.Settled = true
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
				done <- fmt.Errorf("Settle timed out after %d attempts",
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
	ctx.Log.Println("Sending tasks")
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
		if err := ctx.Tee(taskpath, []byte(preamble+script)); err != nil {
			return err
		}
		if err := ctx.SSH("chmod", "755", taskpath).Run(); err != nil {
			return err
		}
	}
	return nil
}

var shunsafe = regexp.MustCompile(`[^\w@%+=:,./-]`)

func shquote(v string) string {
	// Algorithm aped from shlex.py
	if v == "" {
		return "''"
	}
	if !shunsafe.MatchString(v) {
		return v
	}
	return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'"
}

func (ctx *JobContext) SendEnv() error {
	const home = "/home/build"
	ctx.Log.Println("Sending build environment")
	envpath := path.Join(home, ".buildenv")
	env := fmt.Sprintf(`#!/bin/sh
function complete-build() {
	exit 255
}
export JOB_ID=%d
`, ctx.Job.Id)
	if ctx.Manifest.Environment == nil {
		ctx.Manifest.Environment = make(map[string]interface{})
	}
	ctx.Manifest.Environment["JOB_ID"] = ctx.Job.Id
	ctx.Manifest.Environment["JOB_URL"] = fmt.Sprintf(
		"%s/~%s/job/%d", origin, ctx.Job.Username, ctx.Job.Id)
	for key, value := range ctx.Manifest.Environment {
		switch v := value.(type) {
		case bool:
			if v {
				env += fmt.Sprintf("export %s=true\n", key)
			} else {
				env += fmt.Sprintf("export %s=false\n", key)
			}
		case string:
			env += fmt.Sprintf("export %s=%s\n", key, shquote(v))
		case int:
			env += fmt.Sprintf("export %s=%d\n", key, v)
		case float64:
			env += fmt.Sprintf("export %s=%g\n", key, v)
		case []interface{}:
			env += key + "=("
			for i, _item := range v {
				switch item := _item.(type) {
				case string:
					env += fmt.Sprintf("\"%s\"", item)
				}
				if i != len(v)-1 {
					env += " "
				}
			}
			env += ")\n"
		default:
			panic(fmt.Errorf("Unknown environment type %T", value))
		}
	}

	if err := ctx.Tee(envpath, []byte(env)); err != nil {
		return err
	}
	if err := ctx.SSH("chmod", "755", envpath).Run(); err != nil {
		return err
	}

	return nil
}

func (ctx *JobContext) SendSecrets() error {
	if ctx.Manifest.Secrets == nil || len(ctx.Manifest.Secrets) == 0 {
		return nil
	}
	ctx.Log.Println("Sending secrets")
	sshKeys := 0
	for _, uuid := range ctx.Manifest.Secrets {
		ctx.Log.Printf("Resolving secret %s\n", uuid)
		secret, err := GetSecret(ctx.Db, uuid)
		if err != nil {
			return errors.Wrap(err, "GetSecret")
		}
		if secret.UserId != ctx.Job.OwnerId {
			ctx.Log.Printf("Warning: access denied for secret %s\n", uuid)
			continue
		}
		switch secret.SecretType {
		case "ssh_key":
			sshdir := path.Join("/", "home", "build", ".ssh")
			keypath := path.Join(sshdir, uuid)
			if err := ctx.SSH("mkdir", "-p", sshdir).Run(); err != nil {
				return errors.Wrap(err, "mkdir -p ~/.ssh")
			}
			if err := ctx.Tee(keypath, secret.Secret); err != nil {
				return errors.Wrap(err, "tee")
			}
			if err := ctx.SSH("chmod", "600", keypath).Run(); err != nil {
				return errors.Wrap(err, "chmod")
			}
			if sshKeys == 0 {
				if err := ctx.SSH("ln", "-s",
					keypath, path.Join(sshdir, "id_rsa")).Run(); err != nil {

					return errors.Wrap(err, "ln -s id_rsa")
				}
			}
			sshKeys++
		case "pgp_key":
			gpg := ctx.SSH("gpg", "--import")
			pipe, err := gpg.StdinPipe()
			gpg.Stdout = ctx.LogFile
			gpg.Stderr = ctx.LogFile
			if err != nil {
				return errors.Wrap(err, "(gpg --import).StdinPipe")
			}
			if err := gpg.Start(); err != nil {
				return errors.Wrap(err, "(gpg --import).Start")
			}
			if _, err := pipe.Write(secret.Secret); err != nil {
				return errors.Wrap(err, "pipe.Write(secret)")
			}
			pipe.Close()
			if err := gpg.Wait(); err != nil {
				return errors.Wrap(err, "(gpg --import).Wait")
			}
		case "plaintext_file":
			if err := ctx.SSH("mkdir", "-p",
				path.Dir(*secret.Path)).Run(); err != nil {

				return errors.Wrap(err, "mkdir -p $(dirname)")
			}
			if err := ctx.Tee(*secret.Path, secret.Secret); err != nil {
				return errors.Wrap(err, "tee")
			}
			if err := ctx.SSH("chmod", fmt.Sprintf("%o", *secret.Mode),
				*secret.Path).Run(); err != nil {

				return errors.Wrap(err, "chmod")
			}
		default:
			return fmt.Errorf("Unknown secret type %s", secret.SecretType)
		}
	}
	return nil
}

func (ctx *JobContext) ConfigureRepos() error {
	if ctx.Manifest.Repositories == nil || len(ctx.Manifest.Repositories) == 0 {
		return nil
	}
	for name, source := range ctx.Manifest.Repositories {
		ctx.Log.Printf("Adding repository %s\n", name)
		ctrl := ctx.Control(ctx.Context, ctx.Manifest.Image, "add-repo",
			strconv.Itoa(ctx.Port), name, source)
		ctrl.Stdout = ctx.LogFile
		ctrl.Stderr = ctx.LogFile
		if err := ctrl.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *JobContext) CloneRepos() error {
	if ctx.Manifest.Sources == nil || len(ctx.Manifest.Sources) == 0 {
		return nil
	}
	ctx.Log.Println("Cloning repositories")
	for _, srcurl := range ctx.Manifest.Sources {
		scm := "git"
		hash_bits := strings.Split(srcurl, "#")
		ref := ""
		if len(hash_bits) == 2 {
			srcurl = hash_bits[0]
			ref = hash_bits[1]
		}
		repo_name := path.Base(srcurl)
		directory_bits := strings.Split(srcurl, "::")
		if len(directory_bits) == 2 {
			// directory::... form
			srcurl = directory_bits[1]
			if directory_bits[0] != "" {
				repo_name = directory_bits[0]
			}
		}
		purl, err := url.Parse(srcurl)
		if err == nil {
			scheme_bits := strings.Split(purl.Scheme, "+")
			if len(scheme_bits) == 2 {
				// git+https://... form
				scm = scheme_bits[0]
				purl.Scheme = scheme_bits[1]
				srcurl = purl.String()
			}
		}
		if scm == "git" {
			if len(directory_bits) == 1 {
				// we're using the repo name from the url, which may have .git
				repo_name = strings.TrimSuffix(repo_name, ".git")
			}
			git := ctx.SSH("GIT_SSH_COMMAND='ssh -o " +
				"UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no'",
				"git", "clone", srcurl, repo_name)
			git.Stdout = ctx.LogFile
			git.Stderr = ctx.LogFile
			if err := git.Run(); err != nil {
				ctx.Log.Println("Failed to clone repository. " +
					"If this a private repository, make sure you've " +
					"added a suitable SSH key.")
				ctx.Log.Println("https://man.sr.ht/builds.sr.ht/private-repos.md")
				return errors.Wrap(err, "git clone")
			}
			if ref != "" {
				git := ctx.SSH("sh", "-euxc",
					fmt.Sprintf("'cd %s && git checkout -q %s'", repo_name, ref))
				git.Stdout = ctx.LogFile
				git.Stderr = ctx.LogFile
				if err := git.Run(); err != nil {
					return errors.Wrap(err, "git checkout")
				}
			}
			git = ctx.SSH("GIT_SSH_COMMAND='ssh -o " +
				"UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no'",
				"sh", "-euxc",
				fmt.Sprintf("'cd %s && git submodule update --init'", repo_name))
			git.Stdout = ctx.LogFile
			git.Stderr = ctx.LogFile
			if err := git.Run(); err != nil {
				return errors.Wrap(err, "git submodule update")
			}
		} else if scm == "hg" {
			hg := ctx.SSH("hg", "clone",
				"-e", "'ssh -o UserKnownHostsFile=/dev/null " +
				"-o StrictHostKeyChecking=no'", srcurl, repo_name)
			hg.Stdout = ctx.LogFile
			hg.Stderr = ctx.LogFile
			if err := hg.Run(); err != nil {
				ctx.Log.Println("Failed to clone repository. " +
					"If this a private repository, make sure you've " +
					"added a suitable SSH key.")
				ctx.Log.Println("https://man.sr.ht/builds.sr.ht/private-repos.md")
				return errors.Wrap(err, "hg clone")
			}
			if ref != "" {
				hg := ctx.SSH("sh", "-euxc",
					fmt.Sprintf("'cd %s && hg update -y %s'", repo_name, ref))
				hg.Stdout = ctx.LogFile
				hg.Stderr = ctx.LogFile
				if err := hg.Run(); err != nil {
					return errors.Wrap(err, "hg update")
				}
			}
		} else {
			return errors.New("Unknown scm: " + scm)
		}
	}
	return nil
}

func (ctx *JobContext) InstallPackages() error {
	if ctx.Manifest.Packages == nil || len(ctx.Manifest.Packages) == 0 {
		return nil
	}
	ctx.Log.Println("Installing packages")
	ctrl := ctx.Control(ctx.Context, ctx.Manifest.Image, "install",
		strconv.Itoa(ctx.Port), strings.Join(ctx.Manifest.Packages, " "))
	ctrl.Stdout = ctx.LogFile
	ctrl.Stderr = ctx.LogFile
	if err := ctrl.Run(); err != nil {
		return err
	}
	return nil
}

func (ctx *JobContext) RunTasks() error {
	for i, task := range ctx.Manifest.Tasks {
		var (
			err   error
			logfd *os.File
			name  string
			ssh   *exec.Cmd
			tty   *os.File
		)
		for name, _ = range task {
			break
		}

		ctx.Log.Printf("Running task %s\n", name)
		err = ctx.Job.SetTaskStatus(name, "running")
		if err != nil {
			goto fail
		}

		if err = os.Mkdir(path.Join(ctx.LogDir, name), 0755); err != nil {
			goto fail
		}

		ssh = ctx.SSH(path.Join(".", ".tasks", name))
		if logfd, err = os.Create(path.Join(ctx.LogDir, name, "log")); err != nil {
			err = errors.Wrap(err, "Creating log file")
			goto fail
		}
		tty, err = pty.Start(ssh)
		if err != nil {
			err = errors.Wrap(err, "Allocating pty")
			goto fail
		}
		go io.Copy(logfd, tty)

		if err = ssh.Wait(); err != nil {
			exiterr, ok := err.(*exec.ExitError)
			if !ok {
				goto fail
			}
			status, ok := exiterr.Sys().(syscall.WaitStatus)
			if !ok {
				goto fail
			}
			if status.ExitStatus() == 255 {
				ctx.Job.SetTaskStatus(name, "success")
				for i++; i < len(ctx.Manifest.Tasks); i++ {
					for name, _ = range ctx.Manifest.Tasks[i] {
						break
					}
					ctx.Job.SetTaskStatus(name, "skipped")
				}
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

func (ctx *JobContext) UploadArtifacts() error {
	if len(ctx.Manifest.Artifacts) == 0 {
		return nil
	}
	if len(ctx.Manifest.Artifacts) > 8 {
		ctx.Log.Println("Error: no more than 8 artifacts " +
			"per build are accepted.")
		return nil
	}

	var (
		ok        bool
		upstream  string
		accessKey string
		secretKey string
		bucket    string
		prefix    string
	)
	err := errors.New("Build artifacts were requested, but S3 " +
		"is not configured for this build runner.")
	if upstream, ok = config.Get("objects", "s3-upstream");
		!ok || upstream == "" {
		return err
	}
	if accessKey, ok = config.Get("objects", "s3-access-key");
		!ok || accessKey == "" {
		return err
	}
	if secretKey, ok = config.Get("objects", "s3-secret-key");
		!ok || secretKey == "" {
		return err
	}
	if bucket, ok = config.Get("builds.sr.ht::worker", "s3-bucket");
		!ok || bucket == "" {
		return err
	}
	if prefix, ok = config.Get("builds.sr.ht::worker", "s3-prefix"); !ok {
		return err
	}
	mc, err := minio.New(upstream, accessKey, secretKey, true)
	if err != nil {
		return err
	}

	// TODO: Let users configure the bucket location
	if err := mc.MakeBucket(bucket, "us-east-1"); err != nil {
		if exists, err2 := mc.BucketExists(bucket); err2 != nil && exists {
			return err
		}
	}

	random := make([]byte, 8) // Generated to prevent artifact enumeration
	if _, err := rand.Read(random); err != nil {
		return err
	}
	for _, src := range ctx.Manifest.Artifacts {
		ctx.Log.Printf("Uploading %s", src)
		name := path.Join(prefix, "~" + ctx.Job.Username,
			strconv.Itoa(ctx.Job.Id),
			hex.EncodeToString(random),
			filepath.Base(src))
		size, err := ctx.FileSize(shquote(src))
		if err != nil {
			ctx.Log.Printf("Error reading artifact file: %v", err)
			if strings.ContainsRune(src, '~') {
				ctx.Log.Printf("You probably need to remove ~/ from the artifact path.")
			}
			return err
		}
		if size > 1024*1024*1024 { // 1 GiB
			err = errors.New("Artifact exceeds maximum file size")
			ctx.Log.Printf("%v", err)
			return err
		}
		pipe, err := ctx.Download(shquote(src))
		if err != nil {
			ctx.Log.Printf("Error reading artifact file: %v", err)
			return err
		}
		_, err = mc.PutObject(bucket, name, io.LimitReader(pipe, size), size,
			minio.PutObjectOptions{
				ContentType: "application/octet-stream",
			})
		pipe.Close()
		if err != nil {
			return err
		}
		url := fmt.Sprintf("https://%s/%s/%s", upstream, bucket, name)
		err = ctx.Job.InsertArtifact(src, filepath.Base(src), url, size)
		if err != nil {
			return err
		}
		ctx.Log.Println(url)
	}
	return nil
}
