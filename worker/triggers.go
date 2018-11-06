package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ms "github.com/mitchellh/mapstructure"
)

var (
	triggersExecuted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_triggers_executed",
		Help: "The total number of triggers which have been executed",
	})
	webhooksExecuted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_triggers_webhooks",
		Help: "The total number of webhooks which have been delivered",
	})
)

type Trigger struct {
	Action    string
	Condition string
}

func (ctx *JobContext) ProcessTriggers() {
	if ctx.Manifest.Triggers == nil || len(ctx.Manifest.Triggers) == 0 {
		return
	}
	ctx.Log.Println("Processing post-build triggers...")
	for _, def := range ctx.Manifest.Triggers {
		var trigger Trigger
		ms.Decode(def, &trigger)
		failures := map[string]interface{}{
			"failed": nil,
			"timeout": nil,
			"cancelled": nil,
		}
		process := trigger.Condition == "always"
		if _, ok := failures[ctx.Job.Status]; ok {
			process = process || trigger.Condition == "failure"
		}
		if ctx.Job.Status == "success" {
			process = process || trigger.Condition == "success"
		}
		triggers := map[string]func(def map[string]interface{}){
			"webhook": ctx.processWebhook,
		}
		if process {
			if fn, ok := triggers[trigger.Action]; ok {
				fn(def)
				triggersExecuted.Inc()
			} else {
				ctx.Log.Printf("Unknown trigger action '%s'\n", trigger.Action)
			}
		} else {
			ctx.Log.Println("Skipping trigger, condition unmet")
		}
	}
}

func (ctx *JobContext) processWebhook(def map[string]interface{}) {
	type WebhookTrigger struct {
		Url string
	}
	// When updating this, also update buildsrht/types/job.py
	type TaskStatus struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Log    string `json:"log"`
	}
	type JobStatus struct {
		Id       int `json:"id"`
		Status   string `json:"status"`
		SetupLog string `json:"setup_log"`
		Tasks    []TaskStatus `json:"tasks"`
		Note     *string `json:"note"`
		Runner   *string `json:"runner"`
	}

	status := &JobStatus{
		Id: ctx.Job.Id,
		Status: ctx.Job.Status,
		SetupLog: fmt.Sprintf("http://%s/logs/%d/log",
			*ctx.Job.Runner, ctx.Job.Id),
		Note: ctx.Job.Note,
		Runner: ctx.Job.Runner,
	}

	for _, _task := range ctx.Manifest.Tasks {
		var name string
		for name, _ = range _task {
			break
		}
		taskStatus, err := ctx.Job.GetTaskStatus(name)
		if err != nil {
			ctx.Log.Println("Failed to find task status")
			return
		}
		task := TaskStatus{
			Name: name,
			Status: taskStatus,
			Log: fmt.Sprintf("http://%s/logs/%d/%s/log",
				*ctx.Job.Runner, ctx.Job.Id, name),
		}
		status.Tasks = append(status.Tasks, task)
	}

	var trigger WebhookTrigger
	ms.Decode(def, &trigger)

	var (
		data []byte
		err  error
	)
	if data, err = json.Marshal(status); err != nil {
		ctx.Log.Printf("Failed to marshal webhook payload: %v\n", err)
		return
	}

	ctx.Log.Println("Sending webhook...")
	client := &http.Client{Timeout: time.Second*10}
	if resp, err := client.Post(trigger.Url,
		"application/json", bytes.NewReader(data)); err == nil {

		defer resp.Body.Close()
		respData, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 2048))
		ctx.Log.Printf("Webhook response: %d\n", resp.StatusCode)
		if utf8.Valid(respData) {
			ctx.Log.Printf("%s\n", string(respData))
		}
		webhooksExecuted.Inc()
	} else {
		fmt.Printf("Error submitting webhook: %v\n", err)
	}
}
