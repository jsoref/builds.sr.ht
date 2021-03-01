package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/loaders"
	"git.sr.ht/~sircmpwn/core-go/auth"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
)

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
	panic(fmt.Errorf("not implemented"))
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

// Mutation returns api.MutationResolver implementation.
func (r *Resolver) Mutation() api.MutationResolver { return &mutationResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
