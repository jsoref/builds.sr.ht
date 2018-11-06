package main

import (
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
		if r.Method != "POST" {
			w.WriteHeader(405)
			w.Write([]byte("405 method not allowed"))
			return
		}
		switch op {
		case "cancel":
			jobsMutex.Lock()
			defer jobsMutex.Unlock()
			if job, ok := jobs[jobId]; ok {
				job.Cancel()
				job.Job.SetStatus("cancelled")
			} else {
				w.WriteHeader(404)
				w.Write([]byte("404 not found"))
				return
			}
			w.WriteHeader(200)
			w.Write([]byte("cancelled"))
		default:
			w.WriteHeader(404)
			w.Write([]byte("404 not found"))
		}
	})
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
