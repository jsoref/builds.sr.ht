package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	model1 "git.sr.ht/~sircmpwn/core-go/model"
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
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) UserByID(ctx context.Context, id int) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) UserByEmail(ctx context.Context, email string) (*model.User, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Jobs(ctx context.Context, cursor *model1.Cursor) (*model.JobCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Job(ctx context.Context, id *int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

func (r *queryResolver) Secrets(ctx context.Context, cursor *model1.Cursor) (*model.SecretCursor, error) {
	panic(fmt.Errorf("not implemented"))
}

// Mutation returns api.MutationResolver implementation.
func (r *Resolver) Mutation() api.MutationResolver { return &mutationResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
func (r *queryResolver) SecretsForJob(ctx context.Context, job int) ([]model.Secret, error) {
	panic(fmt.Errorf("not implemented"))
}
