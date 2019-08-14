package main

import (
	"encoding/json"
	"net/http"
	"fmt"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func HttpServer() {
	http.HandleFunc("/job/", func(w http.ResponseWriter, r *http.Request) {
		var (
			jobId int
			op    string
		)
		_, err := fmt.Sscanf(r.URL.Path, "/job/%d/%s", &jobId, &op)
		if err != nil {
			w.WriteHeader(404)
			w.Write([]byte("404 not found"))
			return
		}
		switch op {
		case "info":
			if r.Method != "GET" {
				w.WriteHeader(405)
				w.Write([]byte("405 method not allowed"))
				return
			}
			if job, ok := jobs[jobId]; ok {
				w.WriteHeader(200)
				bytes, _ := json.Marshal(struct {
					Deadline int64   `json:"deadline"`
					Manifest string  `json:"manifest"`
					Note     *string `json:"note"`
					OwnerId  int     `json:"owner_id"`
					Port     int     `json:"port"`
					Status   string  `json:"status"`
					Task     int     `json:"task"`
					Tasks    int     `json:"tasks"`
					Username string  `json:"username"`
				} {
					Deadline: job.Deadline.Unix(),
					Manifest: job.Job.Manifest,
					Note:     job.Job.Note,
					OwnerId:  job.Job.OwnerId,
					Port:     job.Port,
					Status:   job.Job.Status,
					Task:     job.Task,
					Tasks:    job.NTasks,
					Username: job.Job.Username,
				})
				w.Write(bytes)
			} else {
				w.WriteHeader(404)
				w.Write([]byte("404 not found"))
			}
		case "cancel":
			fallthrough
		case "terminate":
			if r.Method != "POST" {
				w.WriteHeader(405)
				w.Write([]byte("405 method not allowed"))
				return
			}
			jobsMutex.Lock()
			defer jobsMutex.Unlock()
			if job, ok := jobs[jobId]; ok {
				job.Cancel()
				if op == "cancel" {
					job.Job.SetStatus("cancelled")
				}
			} else {
				w.WriteHeader(404)
				w.Write([]byte("404 not found"))
				return
			}
			w.WriteHeader(200)
			w.Write([]byte("cancelled"))
		case "claim":
			if r.Method != "POST" {
				w.WriteHeader(405)
				w.Write([]byte("405 method not allowed"))
				return
			}
			jobsMutex.Lock()
			defer jobsMutex.Unlock()
			if job, ok := jobs[jobId]; ok {
				job.Claimed = true
				w.WriteHeader(200)
				w.Write([]byte("claimed"))
			} else {
				w.WriteHeader(404)
				w.Write([]byte("404 not found"))
			}
		default:
			w.WriteHeader(404)
			w.Write([]byte("404 not found"))
		}
	})
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
