package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"time"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/loaders"
	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/database"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
)

func (r *jobResolver) Owner(ctx context.Context, obj *model.Job) (model.Entity, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *jobResolver) Group(ctx context.Context, obj *model.Job) (*model.JobGroup, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *jobResolver) Tasks(ctx context.Context, obj *model.Job) ([]*model.Task, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *jobResolver) Artifacts(ctx context.Context, obj *model.Job) ([]*model.Artifact, error) {
	panic(fmt.Errorf("not implemented"))
}

// TODO: Add logs { last128KiB } to the query cost compuation
func (r *jobResolver) Log(ctx context.Context, obj *model.Job) (*model.Log, error) {
	if obj.Runner == nil {
		return nil, nil
	}
	url := fmt.Sprintf("http://%s/logs/%d/log", *obj.Runner, obj.ID)
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
		// TODO: We can trim the response body to 128 KiB ourselves
		panic("Did not expect 200 response from server")
	case http.StatusPartialContent:
		// OK
		break
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

func (r *jobResolver) Secrets(ctx context.Context, obj *model.Job) ([]model.Secret, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Submit(ctx context.Context, manifest string, tags []*string, note *string, secrets *bool, execute *bool) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Start(ctx context.Context, jobID int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Cancel(ctx context.Context, jobID int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateGroup(ctx context.Context, jobIds []*int, triggers []*model.TriggerInput, execute *bool) (*model.JobGroup, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) StartGroup(ctx context.Context, groupID int) (*model.JobGroup, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CancelGroup(ctx context.Context, groupID int) (*model.JobGroup, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) Claim(ctx context.Context, jobID int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateJob(ctx context.Context, jobID int, status model.JobStatus) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) UpdateTask(ctx context.Context, taskID int, status model.TaskStatus) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *mutationResolver) CreateArtifact(ctx context.Context, jobID int, path string, contents string) (*model.Artifact, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Version(ctx context.Context) (*model.Version, error) {
	return &model.Version{
		Major:           0,
		Minor:           0,
		Patch:           0,
		DeprecationDate: nil,
	}, nil
}

func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	user := auth.ForContext(ctx)
	return &model.User{
		ID:       user.UserID,
		Created:  user.Created,
		Updated:  user.Updated,
		Username: user.Username,
		Email:    user.Email,
		URL:      user.URL,
		Location: user.Location,
		Bio:      user.Bio,
	}, nil
}

func (r *queryResolver) UserByID(ctx context.Context, id int) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByID.Load(id)
}

func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByName.Load(username)
}

func (r *queryResolver) Jobs(ctx context.Context, cursor *coremodel.Cursor) (*model.JobCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var jobs []*model.Job
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		job := (&model.Job{}).As(`j`)
		query := database.
			Select(ctx, job).
			From(`job j`).
			Where(`j.owner_id = ?`, auth.ForContext(ctx).UserID)
		jobs, cursor = job.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.JobCursor{jobs, cursor}, nil
}

func (r *queryResolver) Job(ctx context.Context, id *int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Secrets(ctx context.Context, cursor *coremodel.Cursor) (*model.SecretCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *userResolver) Jobs(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.JobCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

// Job returns api.JobResolver implementation.
func (r *Resolver) Job() api.JobResolver { return &jobResolver{r} }

// Mutation returns api.MutationResolver implementation.
func (r *Resolver) Mutation() api.MutationResolver { return &mutationResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

type jobResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
