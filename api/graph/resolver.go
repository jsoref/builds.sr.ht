package graph

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/99designs/gqlgen/graphql"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
)

type Resolver struct{}

func FetchLogs(ctx context.Context, url string) (*model.Log, error) {
	log := &model.Log{FullURL: url}

	// If the user hasn't requested the log body, stop here
	if graphql.GetFieldContext(ctx) != nil {
		found := false
		for _, field := range graphql.CollectFieldsCtx(ctx, nil) {
			if field.Name == "last128KiB" {
				found = true
				break
			}
		}
		if !found {
			return log, nil
		}
	}

	// TODO: It might be possible/desirable to set up an API with the runners
	// we can use to fetch logs in bulk, perhaps gzipped, and set up a loader
	// for it.
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", "bytes=-131072") // Last 128 KiB
	resp, err := http.DefaultClient.Do(req)
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
	b, err := ioutil.ReadAll(limit)
	if err != nil {
		return nil, err
	}
	log.Last128KiB = string(b)

	return log, nil
}

// Starts a job group. Does not authenticate the user.
func StartJobGroupUnsafe(ctx context.Context, tx *sql.Tx, id int) error {
	var manifests []struct {
		ID       int
		Manifest *Manifest
	}

	rows, err := tx.QueryContext(ctx, `
		UPDATE job SET status = 'queued'
		WHERE job_group_id = $1
		RETURNING id, manifest;
	`, id)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id       int
			manifest string
		)
		if err := rows.Scan(&id, &manifest); err != nil {
			return err
		}

		man, err := LoadManifest(manifest)
		if err != nil {
			// Invalid manifests shouldn't make it to the database
			panic(err)
		}

		manifests = append(manifests, struct {
			ID       int
			Manifest *Manifest
		}{
			ID:       id,
			Manifest: man,
		})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	for _, job := range manifests {
		if err := SubmitJob(ctx, job.ID, job.Manifest); err != nil {
			return fmt.Errorf("Failed to submit some jobs: %e", err)
		}
	}

	return nil
}
