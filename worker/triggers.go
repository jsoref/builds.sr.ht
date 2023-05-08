package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"git.sr.ht/~sircmpwn/core-go/crypto"
	"github.com/martinlindhe/base36"
	ms "github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	gomail "gopkg.in/mail.v2"
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
	webhooksFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "buildsrht_triggers_webhooks_failed",
		Help: "The total number of webhooks which were not delivered",
	})
)

// When updating this, also update buildsrht/types/job.py
type TaskStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Log    string `json:"log"`
}

type JobOwner struct {
	CName string `json:"canonical_name"`
	Name  string `json:"name"`
}

type JobStatus struct {
	Id       int          `json:"id"`
	Status   string       `json:"status"`
	SetupLog string       `json:"setup_log"`
	Tasks    []TaskStatus `json:"tasks"`
	Note     *string      `json:"note"`
	Runner   *string      `json:"runner"`
	Owner    JobOwner     `json:"owner"`
}

type JobGroupStatus struct {
	Id    int         `json:"id"`
	Note  *string     `json:"note"`
	Owner JobOwner    `json:"owner"`
	Jobs  []JobStatus `json:"jobs"`
}

type EmailTrigger struct {
	To        *string
	Cc        *string
	InReplyTo *string `mapstructure:"in_reply_to"`
}

type WebhookTrigger struct {
	Url string
}

func (ctx *JobContext) ProcessTriggers() {
	if ctx.Job.JobGroupId != nil {
		ctx.processJobGroupTriggers(*ctx.Job.JobGroupId)
	}

	if ctx.Manifest.Triggers == nil || len(ctx.Manifest.Triggers) == 0 {
		return
	}
	if ctx.ProcessedTriggers {
		// Debounce
		return
	}
	ctx.ProcessedTriggers = true
	ctx.Log.Printf("Processing post-%s triggers...\n", ctx.Job.Status)
	for _, def := range ctx.Manifest.Triggers {
		var trigger Trigger
		ms.Decode(def, &trigger)
		failures := map[string]interface{}{
			"failed":    nil,
			"timeout":   nil,
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
			"email":   ctx.processEmail,
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

func (ctx *JobContext) processEmail(def map[string]interface{}) {
	type EmailTrigger struct {
		To        *string
		Cc        *string
		InReplyTo *string `mapstructure:"in_reply_to"`
	}
	var trigger EmailTrigger
	ms.Decode(def, &trigger)
	if trigger.To == nil {
		ctx.Log.Printf("Expected `to` in email trigger")
		return
	}

	m := gomail.NewMessage()
	sender, err := mail.ParseAddress(ctx.Conf("builds.sr.ht::worker", "trigger-from"))
	if err != nil {
		ctx.Log.Println("Failed to parse sender address")
	}
	m.SetAddressHeader("From", sender.Address, sender.Name)

	subj := "builds.sr.ht"
	if ctx.Job.Tags != nil {
		subj = *ctx.Job.Tags
	}

	m.SetHeader("Message-ID", GenerateMessageID())
	if trigger.InReplyTo != nil {
		m.SetHeader("In-Reply-To", *trigger.InReplyTo)
	}

	m.SetHeader("Subject", fmt.Sprintf(
		"[%s] build %s", subj, ctx.Job.Status))

	recipients, err := mail.ParseAddressList(*trigger.To)
	if err != nil {
		ctx.Log.Println("Failed to parse recipient addresses")
	}
	var toRcpts []string
	for _, rcpt := range recipients {
		toRcpts = append(toRcpts, m.FormatAddress(rcpt.Address, rcpt.Name))
	}
	m.SetHeader("To", toRcpts...)

	if trigger.Cc != nil {
		recipients, err = mail.ParseAddressList(*trigger.Cc)
		if err != nil {
			ctx.Log.Println("Failed to parse recipient addresses")
		}
		var ccRcpts []string
		for _, rcpt := range recipients {
			ccRcpts = append(ccRcpts, m.FormatAddress(rcpt.Address, rcpt.Name))
		}
		m.SetHeader("Cc", ccRcpts...)
	}

	var taskBuf bytes.Buffer
	for _, _task := range ctx.Manifest.Tasks {
		var name string
		for name, _ = range _task {
			break
		}
		if strings.HasPrefix(name, "_") {
			continue
		}
		taskStatus, err := ctx.Job.GetTaskStatus(name)
		if err != nil {
			ctx.Log.Println("Failed to find task status")
			return
		}
		statusChar := '-'
		if taskStatus == "success" {
			statusChar = '✓'
		} else if taskStatus == "failed" {
			statusChar = '✗'
		}
		taskBuf.WriteString(fmt.Sprintf("%c %s ", statusChar, name))
	}
	type TemplateContext struct {
		Duration string
		Origin   string
		Job      *Job
		Status   string
		Tasks    string
	}
	tmpl, err := template.New("email").Parse(
		`{{if .Job.Tags}}{{.Job.Tags}}{{else}}Job{{end}} #{{.Job.Id}}: {{.Status}} in {{.Duration}}

{{if .Job.Note}}{{.Job.Note}}

{{end}}{{.Origin}}/~{{.Job.Username}}/job/{{.Job.Id}}

{{.Tasks}}`)
	if err != nil {
		ctx.Log.Printf("Error rendering email: %v\n", err)
		return
	}
	var buf bytes.Buffer
	tmpl.Execute(&buf, &TemplateContext{
		Duration: time.Since(ctx.Job.Created).Truncate(time.Second).String(),
		Job:      ctx.Job,
		Origin:   ctx.Conf("builds.sr.ht", "origin"),
		Status:   strings.ToUpper(ctx.Job.Status),
		Tasks:    taskBuf.String(),
	})
	// TODO: PGP
	m.SetBody("text/plain", buf.String())

	port, _ := strconv.Atoi(ctx.Conf("mail", "smtp-port"))
	d := gomail.NewDialer(ctx.Conf("mail", "smtp-host"), port,
		ctx.Conf("mail", "smtp-user"), ctx.Conf("mail", "smtp-password"))
	// TODO: TLS
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	if err := d.DialAndSend(m); err != nil {
		ctx.Log.Printf("Error sending email: %v\n", err)
		return
	}
	ctx.Log.Printf("Sent build results email to %s", *trigger.To)
}

func (ctx *JobContext) processWebhook(def map[string]interface{}) {
	status := &JobStatus{
		Id:     ctx.Job.Id,
		Status: ctx.Job.Status,
		SetupLog: fmt.Sprintf("http://%s/logs/%d/log",
			*ctx.Job.Runner, ctx.Job.Id),
		Note:   ctx.Job.Note,
		Runner: ctx.Job.Runner,
		Owner: JobOwner{
			CName: "~" + ctx.Job.Username,
			Name:  ctx.Job.Username,
		},
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
			Name:   name,
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

	nonce, sig := crypto.SignWebhook(data)

	ctx.Log.Println("Sending webhook...")
	client := &http.Client{Timeout: time.Second * 10}
	req, err := http.NewRequest("POST", trigger.Url, bytes.NewReader(data))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-Payload-Nonce", nonce)
	req.Header.Add("X-Payload-Signature", sig)

	if resp, err := client.Do(req); err == nil {
		defer resp.Body.Close()
		respData, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 2048))
		ctx.Log.Printf("Webhook response: %d\n", resp.StatusCode)
		if utf8.Valid(respData) {
			ctx.Log.Printf("%s\n", string(respData))
		}
		webhooksExecuted.Inc()
	} else {
		ctx.Log.Printf("Error submitting webhook: %v\n", err)
		webhooksFailed.Inc()
	}
}

func (ctx *JobContext) processJobGroupTriggers(groupId int) {
	group, err := GetJobGroup(ctx.Db, groupId)
	if err != nil {
		ctx.Log.Printf("Failed to fetch job group details: %e", err)
		return
	}
	if !group.Complete {
		return
	}

	triggers, err := group.GetTriggers()
	if err != nil {
		ctx.Log.Printf("Failed to fetch job group triggers: %e", err)
		return
	}
	if len(triggers) == 0 {
		return
	}

	err = group.GetJobs()
	if err != nil {
		ctx.Log.Printf("Failed to fetch job group members: %e", err)
		return
	}

	done := true
	status := "success"
	for _, job := range group.Jobs {
		// The group is only considered successful if all jobs are successful
		if job.Status != "success" {
			status = job.Status
		}
		if status == "running" || status == "queued" || status == "pending" {
			done = false
		}
	}

	if !done {
		return
	}

	ctx.Log.Printf("Processing post-%s triggers for job group...\n", status)
	for _, trigger := range triggers {
		var def map[string]interface{}
		err = json.Unmarshal([]byte(trigger.Details), &def)
		if err != nil {
			ctx.Log.Printf("Failed to unmarshal trigger details: %e", err)
			return
		}
		failures := map[string]interface{}{
			"failed":    nil,
			"timeout":   nil,
			"cancelled": nil,
		}
		process := trigger.Condition == "always"
		if _, ok := failures[status]; ok {
			process = process || trigger.Condition == "failure"
		}
		if ctx.Job.Status == "success" {
			process = process || trigger.Condition == "success"
		}
		triggers := map[string]func(*JobGroup, string, map[string]interface{}){
			"email": ctx.processGroupEmail,
			//"webhook": ctx.processGroupWebhook, TODO
		}
		if process {
			if fn, ok := triggers[trigger.Action]; ok {
				fn(group, status, def)
				triggersExecuted.Inc()
			} else {
				ctx.Log.Printf("Unknown trigger action '%s'\n", trigger.Action)
			}
		} else {
			ctx.Log.Println("Skipping trigger, condition unmet")
		}
	}
}

func (ctx *JobContext) processGroupEmail(group *JobGroup, status string,
	def map[string]interface{}) {
	// TODO: merge some of this with the other email trigger

	var trigger EmailTrigger
	ms.Decode(def, &trigger)
	if trigger.To == nil {
		ctx.Log.Printf("Expected `to` in email trigger")
		return
	}

	m := gomail.NewMessage()
	sender, err := mail.ParseAddress(ctx.Conf("builds.sr.ht::worker", "trigger-from"))
	if err != nil {
		ctx.Log.Println("Failed to parse sender address")
	}
	m.SetAddressHeader("From", sender.Address, sender.Name)

	var (
		jobTags []string
		tags    []string
	)
	maxTags := 0
	maxStatus := 0
	for _, job := range group.Jobs {
		if job.Tags != nil {
			jt := strings.Split(*job.Tags, "/")
			if tags == nil {
				tags = jt
			} else {
				for i, _ := range jt {
					if i < len(tags) && tags[i] != jt[i] {
						tags = tags[:i]
						break
					}
				}
			}

			jobTags = append(jobTags, *job.Tags)
			if len(*job.Tags) > maxTags {
				maxTags = len(*job.Tags)
			}
			if len(job.Status) > maxStatus {
				maxStatus = len(job.Status)
			}
		}
	}
	var (
		tag  string = ""
		subj string = "builds.sr.ht"
	)
	if tags != nil {
		tag = strings.Join(tags, "/")
		subj = tag
	}

	m.SetHeader("Message-ID", GenerateMessageID())
	if trigger.InReplyTo != nil {
		m.SetHeader("In-Reply-To", *trigger.InReplyTo)
	}

	m.SetHeader("Subject", fmt.Sprintf(
		"[%s] build %s", subj, status))

	recipients, err := mail.ParseAddressList(*trigger.To)
	if err != nil {
		ctx.Log.Println("Failed to parse recipient addresses")
	}
	var toRcpts []string
	for _, rcpt := range recipients {
		toRcpts = append(toRcpts, m.FormatAddress(rcpt.Address, rcpt.Name))
	}
	m.SetHeader("To", toRcpts...)

	if trigger.Cc != nil {
		recipients, err = mail.ParseAddressList(*trigger.Cc)
		if err != nil {
			ctx.Log.Println("Failed to parse recipient addresses")
		}
		var ccRcpts []string
		for _, rcpt := range recipients {
			ccRcpts = append(ccRcpts, m.FormatAddress(rcpt.Address, rcpt.Name))
		}
		m.SetHeader("Cc", ccRcpts...)
	}

	var jobsBuf bytes.Buffer
	for _, job := range group.Jobs {
		statusChar := '-'
		if job.Status == "success" {
			statusChar = '✓'
		} else {
			statusChar = '✗'
		}
		origin := ctx.Conf("builds.sr.ht", "origin")
		url := fmt.Sprintf("%s/~%s/job/%d", origin, job.Username, job.Id)
		if job.Tags != nil {
			jobsBuf.WriteString(fmt.Sprintf("%c #%d %-"+
				strconv.FormatInt(int64(maxStatus), 10)+"s %-"+
				strconv.FormatInt(int64(maxTags), 10)+"s %s\n",
				statusChar, job.Id, strings.ToUpper(job.Status), *job.Tags, url))
		} else {
			jobsBuf.WriteString(fmt.Sprintf("%c #%d %-"+
				strconv.FormatInt(int64(maxStatus), 10)+"s %-"+
				strconv.FormatInt(int64(maxTags), 10)+"s %s\n",
				statusChar, job.Id, strings.ToUpper(job.Status), job.Image, url))
		}
	}
	type TemplateContext struct {
		Duration string
		Origin   string
		Group    *JobGroup
		Status   string
		Jobs     string
		Tag      string
	}
	// TODO: Use job group tags here
	tmpl, err := template.New("group-email").Parse(
		`{{if .Tag}}{{.Tag}}{{else}}Job group{{end}}: {{.Status}} in {{.Duration}}

{{if .Group.Note}}{{.Group.Note}}{{end}}

{{.Jobs}}`)
	if err != nil {
		ctx.Log.Printf("Error rendering email: %v\n", err)
		return
	}
	var buf bytes.Buffer
	tmpl.Execute(&buf, &TemplateContext{
		Duration: time.Since(group.Created).Truncate(time.Second).String(),
		Group:    group,
		Status:   strings.ToUpper(status),
		Jobs:     jobsBuf.String(),
		Tag:      tag,
	})
	// TODO: PGP
	m.SetBody("text/plain", buf.String())

	port, _ := strconv.Atoi(ctx.Conf("mail", "smtp-port"))
	d := gomail.NewDialer(ctx.Conf("mail", "smtp-host"), port,
		ctx.Conf("mail", "smtp-user"), ctx.Conf("mail", "smtp-password"))
	// TODO: TLS
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	if err := d.DialAndSend(m); err != nil {
		ctx.Log.Printf("Error sending email: %v\n", err)
		return
	}
	ctx.Log.Printf("Sent build results email to %s", *trigger.To)
}

// Generates an RFC 2822-compliant Message-Id based on the informational draft
// "Recommendations for generating Message IDs", for lack of a better
// authoritative source.
func GenerateMessageID() string {
	var (
		now   bytes.Buffer
		nonce []byte = make([]byte, 8)
	)
	binary.Write(&now, binary.BigEndian, time.Now().UnixNano())
	rand.Read(nonce)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "localhost"
	}
	return fmt.Sprintf("<%s.%s@%s>",
		base36.EncodeBytes(now.Bytes()),
		base36.EncodeBytes(nonce),
		hostname)
}
