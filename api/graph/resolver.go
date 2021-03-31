package graph

//go:generate go run github.com/99designs/gqlgen

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
)

type Resolver struct{}

func FetchLogs(url string) (*model.Log, error) {
	// TODO: It might be possible/desirable to set up an API with the runners
	// we can use to fetch logs in bulk, perhaps gzipped, and set up a loader
	// for it.
	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", "bytes=-131072") // Last 128 KiB
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusPartialContent:
		// OK
		break
	case http.StatusNotFound:
		return nil, nil
	default:
		return nil, fmt.Errorf("Unexpected response from build runner: %s", resp.Status)
	}
	limit := io.LimitReader(resp.Body, 131072)
	log, err := io.ReadAll(limit)
	if err != nil {
		return nil, err
	}
	return &model.Log {
		Last128KiB: string(log),
		FullURL:    url,
	}, nil
}
