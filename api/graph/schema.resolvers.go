package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/loaders"
	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
	yaml "gopkg.in/yaml.v2"
)

func (r *jobResolver) Owner(ctx context.Context, obj *model.Job) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

func (r *jobResolver) Group(ctx context.Context, obj *model.Job) (*model.JobGroup, error) {
	if obj.JobGroupID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).JobGroupsByID.Load(*obj.JobGroupID)
}

func (r *jobResolver) Tasks(ctx context.Context, obj *model.Job) ([]*model.Task, error) {
	var tasks []*model.Task

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		task := (&model.Task{}).As(`t`)
		rows, err := database.
			Select(ctx, task).
			From(`task t`).
			Join(`job j ON j.id = t.job_id`).
			Columns(`j.runner`).
			Where(`t.job_id = ?`, obj.ID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var task model.Task
			if err := rows.Scan(append(database.Scan(ctx, &task), &task.Runner)...); err != nil {
				panic(err)
			}
			tasks = append(tasks, &task)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return tasks, nil
}

func (r *jobResolver) Artifacts(ctx context.Context, obj *model.Job) ([]*model.Artifact, error) {
	var artifacts []*model.Artifact
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		artifact := (&model.Artifact{}).As(`a`)
		rows, err := database.
			Select(ctx, artifact).
			From(`artifact a`).
			Where(`a.job_id = ?`, obj.ID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			panic(err)
		}
		defer rows.Close()
		for rows.Next() {
			var artifact model.Artifact
			if err := rows.Scan(database.Scan(ctx, &artifact)...); err != nil {
				panic(err)
			}
			artifacts = append(artifacts, &artifact)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (r *jobResolver) Log(ctx context.Context, obj *model.Job) (*model.Log, error) {
	if obj.Runner == nil {
		return nil, nil
	}
	url := fmt.Sprintf("http://%s/logs/%d/log", *obj.Runner, obj.ID)
	return FetchLogs(url)
}

func (r *jobResolver) Secrets(ctx context.Context, obj *model.Job) ([]model.Secret, error) {
	var secrets []model.Secret

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx,
			`SELECT manifest FROM job WHERE id = $1`, obj.ID)

		var rawManifest string
		if err := row.Scan(&rawManifest); err != nil {
			return err
		}

		type Manifest struct {
			Secrets []string `yaml:"secrets"`
		}

		var manifest Manifest
		if err := yaml.Unmarshal([]byte(rawManifest), &manifest); err != nil {
			return err
		}

		secret := (&model.RawSecret{}).As(`sec`)
		rows, err := database.
			Select(ctx, secret).
			From(`secret sec`).
			Where(sq.Expr(`sec.uuid = ANY(?)`, pq.Array(manifest.Secrets))).
			Where(`sec.user_id = ? AND sec.user_id = ?`,
				obj.OwnerID, auth.ForContext(ctx).UserID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var sec model.RawSecret
			if err := rows.Scan(database.Scan(ctx, &sec)...); err != nil {
				panic(err)
			}
			secrets = append(secrets, sec.ToSecret())
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return secrets, nil
}

func (r *jobGroupResolver) Owner(ctx context.Context, obj *model.JobGroup) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

func (r *jobGroupResolver) Jobs(ctx context.Context, obj *model.JobGroup) ([]*model.Job, error) {
	var jobs []*model.Job
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		job := (&model.Job{}).As(`j`)
		rows, err := database.
			Select(ctx, job).
			From(`job j`).
			Where(`j.job_group_id = ?`, obj.ID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var job model.Job
			if err := rows.Scan(database.Scan(ctx, &job)...); err != nil {
				panic(err)
			}
			jobs = append(jobs, &job)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *jobGroupResolver) Triggers(ctx context.Context, obj *model.JobGroup) ([]model.Trigger, error) {
	var triggers []model.Trigger
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		trigger := (&model.RawTrigger{}).As(`t`)
		rows, err := database.
			Select(ctx, trigger).
			From(`trigger t`).
			Where(`t.job_group_id = ?`, obj.ID).
			RunWith(tx).
			QueryContext(ctx)
		if err != nil {
			panic(err)
		}
		defer rows.Close()

		for rows.Next() {
			var trigger model.RawTrigger
			if err := rows.Scan(database.Scan(ctx, &trigger)...); err != nil {
				panic(err)
			}
			triggers = append(triggers, trigger.ToTrigger())
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return triggers, nil
}

func (r *mutationResolver) Submit(ctx context.Context, manifest string, tags []string, note *string, secrets *bool, execute *bool) (*model.Job, error) {
	man, err := LoadManifest(manifest)
	if err != nil {
		return nil, err
	}
	conf := config.ForContext(ctx)
	user := auth.ForContext(ctx)

	allowFree, _ := conf.Get("builds.sr.ht", "allow-free")
	if allowFree != "yes" {
		if user.UserType != "admin" &&
			user.UserType != "active_free" &&
			user.UserType != "active_non_paying" {
			return nil, fmt.Errorf("A paid account is required to submit builds")
		}
	}

	var job model.Job
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		sec := true
		if secrets != nil {
			sec = *secrets
		}
		status := "queued"
		if execute != nil && !*execute {
			status = "pending"
		}
		var tagbuf bytes.Buffer
		for i, tag := range tags {
			tagbuf.WriteString(tag)
			if i + 1 < len(tags) {
				tagbuf.WriteString("/")
			}
		}

		// TODO: Refactor tags into a pg array
		row := tx.QueryRowContext(ctx, `INSERT INTO job (
			created, updated,
			manifest, owner_id, secrets, note, tags, image, status
		) VALUES (
			NOW() at time zone 'utc',
			NOW() at time zone 'utc',
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING
			id, created, updated, manifest, note, image, runner, owner_id,
			tags, status
		`, manifest, user.UserID, sec, note, tagbuf.String(), man.Image, status)

		if err := row.Scan(&job.ID, &job.Created, &job.Updated, &job.Manifest,
			&job.Note, &job.Image, &job.Runner, &job.OwnerID, &job.RawTags,
			&job.RawStatus); err != nil {
			return err
		}

		for _, task := range man.Tasks {
			var name string
			for key, _ := range task {
				name = key
				break
			}

			_, err := tx.ExecContext(ctx, `INSERT INTO task (
				created, updated, name, status, job_id
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, 'pending', $2
			)`, name, job.ID)

			if err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if execute == nil || *execute {
		if err := SubmitJob(ctx, job.ID, man); err != nil {
			panic(err)
		}
	}

	return &job, nil
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

func (r *pGPKeyResolver) PrivateKey(ctx context.Context, obj *model.PGPKey) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
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

func (r *queryResolver) Job(ctx context.Context, id int) (*model.Job, error) {
	return loaders.ForContext(ctx).JobsByID.Load(id)
}

func (r *queryResolver) Secrets(ctx context.Context, cursor *coremodel.Cursor) (*model.SecretCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var secrets []model.Secret
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		secret := (&model.RawSecret{}).As(`sec`)
		query := database.
			Select(ctx, secret).
			From(`secret sec`).
			Where(`sec.user_id = ?`, auth.ForContext(ctx).UserID)
		secrets, cursor = secret.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.SecretCursor{secrets, cursor}, nil
}

func (r *sSHKeyResolver) PrivateKey(ctx context.Context, obj *model.SSHKey) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
	panic(fmt.Errorf("not implemented"))
}

func (r *secretFileResolver) Data(ctx context.Context, obj *model.SecretFile) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
	panic(fmt.Errorf("not implemented"))
}

func (r *taskResolver) Log(ctx context.Context, obj *model.Task) (*model.Log, error) {
	if obj.Runner == nil {
		return nil, nil
	}
	url := fmt.Sprintf("http://%s/logs/%d/%s/log", *obj.Runner, obj.JobID, obj.Name)
	return FetchLogs(url)
}

func (r *taskResolver) Job(ctx context.Context, obj *model.Task) (*model.Job, error) {
	return loaders.ForContext(ctx).JobsByID.Load(obj.JobID)
}

func (r *userResolver) Jobs(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.JobCursor, error) {
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
			Where(`j.owner_id = ?`, obj.ID)
		jobs, cursor = job.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.JobCursor{jobs, cursor}, nil
}

// Job returns api.JobResolver implementation.
func (r *Resolver) Job() api.JobResolver { return &jobResolver{r} }

// JobGroup returns api.JobGroupResolver implementation.
func (r *Resolver) JobGroup() api.JobGroupResolver { return &jobGroupResolver{r} }

// Mutation returns api.MutationResolver implementation.
func (r *Resolver) Mutation() api.MutationResolver { return &mutationResolver{r} }

// PGPKey returns api.PGPKeyResolver implementation.
func (r *Resolver) PGPKey() api.PGPKeyResolver { return &pGPKeyResolver{r} }

// Query returns api.QueryResolver implementation.
func (r *Resolver) Query() api.QueryResolver { return &queryResolver{r} }

// SSHKey returns api.SSHKeyResolver implementation.
func (r *Resolver) SSHKey() api.SSHKeyResolver { return &sSHKeyResolver{r} }

// SecretFile returns api.SecretFileResolver implementation.
func (r *Resolver) SecretFile() api.SecretFileResolver { return &secretFileResolver{r} }

// Task returns api.TaskResolver implementation.
func (r *Resolver) Task() api.TaskResolver { return &taskResolver{r} }

// User returns api.UserResolver implementation.
func (r *Resolver) User() api.UserResolver { return &userResolver{r} }

type jobResolver struct{ *Resolver }
type jobGroupResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type pGPKeyResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type sSHKeyResolver struct{ *Resolver }
type secretFileResolver struct{ *Resolver }
type taskResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
