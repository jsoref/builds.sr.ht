package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.39

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/account"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/api"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/loaders"
	"git.sr.ht/~sircmpwn/builds.sr.ht/api/webhooks"
	"git.sr.ht/~sircmpwn/core-go/auth"
	"git.sr.ht/~sircmpwn/core-go/config"
	"git.sr.ht/~sircmpwn/core-go/database"
	coremodel "git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/core-go/server"
	"git.sr.ht/~sircmpwn/core-go/valid"
	corewebhooks "git.sr.ht/~sircmpwn/core-go/webhooks"
	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/lib/pq"
	yaml "gopkg.in/yaml.v2"
)

// Owner is the resolver for the owner field.
func (r *jobResolver) Owner(ctx context.Context, obj *model.Job) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

// Group is the resolver for the group field.
func (r *jobResolver) Group(ctx context.Context, obj *model.Job) (*model.JobGroup, error) {
	if obj.JobGroupID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).JobGroupsByID.Load(*obj.JobGroupID)
}

// Tasks is the resolver for the tasks field.
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

// Artifacts is the resolver for the artifacts field.
func (r *jobResolver) Artifacts(ctx context.Context, obj *model.Job) ([]*model.Artifact, error) {
	var artifacts []*model.Artifact
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		artifact := (&model.Artifact{}).As(`a`)
		rows, err := database.
			SelectAll(artifact).
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
			if err := rows.Scan(database.ScanAll(&artifact)...); err != nil {
				panic(err)
			}

			if time.Now().UTC().After(artifact.Created.Add(90 * 24 * time.Hour)) {
				artifact.URL = nil
			}

			artifacts = append(artifacts, &artifact)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return artifacts, nil
}

// Log is the resolver for the log field.
func (r *jobResolver) Log(ctx context.Context, obj *model.Job) (*model.Log, error) {
	if obj.Runner == nil {
		return nil, nil
	}
	return FetchLogs(ctx, *obj.Runner, obj.ID, "")
}

// Secrets is the resolver for the secrets field.
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

// Owner is the resolver for the owner field.
func (r *jobGroupResolver) Owner(ctx context.Context, obj *model.JobGroup) (model.Entity, error) {
	return loaders.ForContext(ctx).UsersByID.Load(obj.OwnerID)
}

// Jobs is the resolver for the jobs field.
func (r *jobGroupResolver) Jobs(ctx context.Context, obj *model.JobGroup) ([]*model.Job, error) {
	user := auth.ForContext(ctx)
	var jobs []*model.Job
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		job := (&model.Job{}).As(`j`)
		rows, err := database.
			Select(ctx, job).
			From(`job j`).
			Where(sq.And{
				sq.Expr(`j.job_group_id = ?`, obj.ID),
				sq.Or{
					sq.Expr(`j.owner_id = ?`, user.UserID),
					sq.Expr(`j.visibility = 'PUBLIC'`),
				},
			}).
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

// Triggers is the resolver for the triggers field.
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

// Submit is the resolver for the submit field.
func (r *mutationResolver) Submit(ctx context.Context, manifest string, tags []string, note *string, secrets *bool, execute *bool, visibility *model.Visibility) (*model.Job, error) {
	man, err := LoadManifest(manifest)
	if err != nil {
		return nil, err
	}
	conf := config.ForContext(ctx)
	user := auth.ForContext(ctx)

	vis := model.VisibilityUnlisted
	if visibility != nil {
		vis = *visibility
	}

	allowFree, _ := conf.Get("builds.sr.ht", "allow-free")
	if allowFree != "yes" {
		if user.UserType != "admin" &&
			user.UserType != "active_free" &&
			user.UserType != "active_paying" {
			return nil, fmt.Errorf("A paid account is required to submit builds")
		}
	}

	secretsErr := user.Access("SECRETS", auth.RO)

	var sec bool
	if secrets != nil {
		sec = *secrets
	} else {
		sec = (len(man.Secrets) > 0 || man.OAuth != "") && secretsErr == nil
	}

	if sec && secretsErr != nil {
		return nil, secretsErr
	}

	if man.OAuth != "" {
		_, err := auth.DecodeGrants(ctx, man.OAuth)
		if err != nil {
			return nil, err
		}
	}

	var job model.Job
	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		tags := strings.Join(tags, "/")

		// TODO: Refactor tags into a pg array
		row := tx.QueryRowContext(ctx, `INSERT INTO job (
			created, updated,
			manifest, owner_id, secrets, note, tags, image, status, visibility
		) VALUES (
			NOW() at time zone 'utc',
			NOW() at time zone 'utc',
			$1, $2, $3, $4, $5, $6, 'pending', $7
		) RETURNING
			id, created, updated, manifest, note, image, runner, owner_id,
			tags, status, visibility
		`, manifest, user.UserID, sec, note, tags, man.Image, vis)

		if err := row.Scan(&job.ID, &job.Created, &job.Updated, &job.Manifest,
			&job.Note, &job.Image, &job.Runner, &job.OwnerID, &job.RawTags,
			&job.RawStatus, &job.Visibility); err != nil {
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

	webhooks.DeliverUserJobEvent(ctx, model.WebhookEventJobCreated, &job)
	return &job, nil
}

// Start is the resolver for the start field.
func (r *mutationResolver) Start(ctx context.Context, jobID int) (*model.Job, error) {
	var job model.Job

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			UPDATE job
			SET status = 'queued'
			WHERE
				id = $1 AND
				status = 'pending' AND
				owner_id = $2
			RETURNING
				id, created, updated, manifest, note, image, runner, owner_id,
				tags, status;
		`, jobID, auth.ForContext(ctx).UserID)
		if err := row.Scan(&job.ID, &job.Created, &job.Updated, &job.Manifest,
			&job.Note, &job.Image, &job.Runner, &job.OwnerID, &job.RawTags,
			&job.RawStatus); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("Found no pending jobs with this ID on your account")
			}
			return err
		}

		man, err := LoadManifest(job.Manifest)
		if err != nil {
			// Invalid manifests should not have made it to the database
			panic(err)
		}
		if err := SubmitJob(ctx, job.ID, man); err != nil {
			panic(err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &job, nil
}

// Cancel is the resolver for the cancel field.
func (r *mutationResolver) Cancel(ctx context.Context, jobID int) (*model.Job, error) {
	job := (&model.Job{}).As(`j`)

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := database.
			Select(ctx, job).
			From(`job j`).
			Columns(`j.runner`).
			Where(`j.id = ?`, jobID).
			Where(`j.owner_id = ?`, auth.ForContext(ctx).UserID).
			Where(`j.status = 'running'`).
			RunWith(tx).
			QueryRowContext(ctx)

		var runner string
		if err := row.Scan(append(
			database.Scan(ctx, job), &runner)...); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("Found no running jobs for your account with this ID")
			}
			return err
		}

		resp, err := http.Post(fmt.Sprintf("http://%s/job/%d/cancel",
			runner, job.ID), "application/json", bytes.NewReader([]byte{}))
		if err != nil {
			return err
		}

		// If the job was found like this in the database, but the
		// runner does not know about it, then something is wrong (e.g.
		// the runner crashed and rebooted). Go ahead and set it to
		// cancelled, so that it does not block anything.
		if resp.StatusCode != 200 && resp.StatusCode != 404 {
			return fmt.Errorf("Failed to cancel job")
		}

		job.RawStatus = "cancelled"
		return nil
	}); err != nil {
		return nil, err
	}

	return job, nil
}

// CreateGroup is the resolver for the createGroup field.
func (r *mutationResolver) CreateGroup(ctx context.Context, jobIds []int, triggers []*model.TriggerInput, execute *bool, note *string) (*model.JobGroup, error) {
	var group model.JobGroup

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO job_group (
				created, updated, owner_id, note
			) VALUES (
				NOW() at time zone 'utc',
				NOW() at time zone 'utc',
				$1, $2
			) RETURNING
				id, created, note, owner_id
			`, auth.ForContext(ctx).UserID, note)
		if err := row.Scan(&group.ID, &group.Created,
			&group.Note, &group.OwnerID); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE job
			SET job_group_id = $1
			WHERE
				job_group_id IS NULL AND
				status = 'pending' AND
				id = ANY($2) AND owner_id = $3;
			`, group.ID, pq.Array(jobIds), auth.ForContext(ctx).UserID)
		if err != nil {
			panic(err)
		}

		affected, err := result.RowsAffected()
		if err != nil {
			panic(err)
		}

		if affected != int64(len(jobIds)) {
			return fmt.Errorf("Invalid list of job IDs. All jobs must be owned by you, not assigned to another job group, and in the pending state.")
		}

		for _, trigger := range triggers {
			if trigger == nil {
				panic("GQL schema invariant broken: nil trigger")
			}

			type EmailDetails struct {
				Action    string  `json:"action"`
				Condition string  `json:"condition"`
				To        string  `json:"to"`
				Cc        *string `json:"cc"`
				InReplyTo *string `json:"in_reply_to"`
			}
			type WebhookDetails struct {
				Action    string `json:"action"`
				Condition string `json:"condition"`
				URL       string `json:"url"`
			}

			var (
				details     string
				triggerType string
			)

			// TODO: Drop job_id column from triggers (unused)
			switch trigger.Type {
			case model.TriggerTypeEmail:
				triggerType = "email"
				email := EmailDetails{
					Action:    "email",
					Condition: strings.ToLower(trigger.Condition.String()),
					To:        trigger.Email.To,
					Cc:        trigger.Email.Cc,
					InReplyTo: trigger.Email.InReplyTo,
				}
				buf, err := json.Marshal(&email)
				if err != nil {
					panic(err)
				}
				details = string(buf)
			case model.TriggerTypeWebhook:
				triggerType = "webhook"
				webhook := WebhookDetails{
					Action:    "webhook",
					Condition: strings.ToLower(trigger.Condition.String()),
					URL:       trigger.Webhook.URL,
				}
				buf, err := json.Marshal(&webhook)
				if err != nil {
					panic(err)
				}
				details = string(buf)
			default:
				panic("GQL schema invariant broken: invalid trigger type")
			}

			_, err = tx.ExecContext(ctx, `
				INSERT INTO trigger (
					created, updated, details, condition, trigger_type,
					job_group_id
				) VALUES (
					NOW() at time zone 'utc',
					NOW() at time zone 'utc',
					$1, $2, $3, $4
				)`,
				details, strings.ToLower(trigger.Condition.String()),
				triggerType, group.ID)

			if err != nil {
				return err
			}
		}

		if execute != nil && !*execute {
			return nil
		}

		return StartJobGroupUnsafe(ctx, tx, group.ID, group.OwnerID)
	}); err != nil {
		return nil, err
	}

	return &group, nil
}

// StartGroup is the resolver for the startGroup field.
func (r *mutationResolver) StartGroup(ctx context.Context, groupID int) (*model.JobGroup, error) {
	var group model.JobGroup

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			UPDATE job_group SET updated = NOW() at time zone 'utc'
			WHERE id = $1 AND owner_id = $2
			RETURNING id, created, note, owner_id
			`, groupID, auth.ForContext(ctx).UserID)
		if err := row.Scan(&group.ID, &group.Created,
			&group.Note, &group.OwnerID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("Found no job group by this ID for your account")
			}
			return err
		}

		return StartJobGroupUnsafe(ctx, tx, groupID, group.OwnerID)
	}); err != nil {
		return nil, err
	}

	return &group, nil
}

// ShareSecret is the resolver for the shareSecret field.
func (r *mutationResolver) ShareSecret(ctx context.Context, uuid string, user string) (model.Secret, error) {
	var sec model.Secret

	valid := valid.New(ctx)
	target, err := loaders.ForContext(ctx).UsersByName.Load(user)
	if err != nil || target == nil {
		valid.Error("No such user").WithField("username")
	}
	if !valid.Ok() {
		return nil, nil
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO secret (
				user_id,
				created,
				updated,
				uuid,
				name,
				from_user_id,
				secret_type,
				secret,
				path,
				mode
			)
			SELECT
				$3,
				created,
				updated,
				uuid,
				name,
				$1,
				secret_type,
				secret,
				path,
				mode
			FROM secret
			WHERE
				user_id = $1 AND uuid = $2
			RETURNING
				id,
				created,
				uuid,
				name,
				from_user_id,
				secret_type,
				secret bytea,
				path,
				mode;
		`, auth.ForContext(ctx).UserID, uuid, target.ID)

		var raw model.RawSecret
		if err := row.Scan(
			&raw.ID,
			&raw.Created,
			&raw.UUID,
			&raw.Name,
			&raw.FromUserID,
			&raw.SecretType,
			&raw.Secret,
			&raw.Path,
			&raw.Mode,
		); err != nil {
			return err
		}
		sec = raw.ToSecret()
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			valid.Error("No such secret").WithField("uuid")
		}
		if !valid.Ok() {
			return nil, nil
		}
		return nil, err
	}

	return sec, nil
}

// Claim is the resolver for the claim field.
func (r *mutationResolver) Claim(ctx context.Context, jobID int) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

// UpdateJob is the resolver for the updateJob field.
func (r *mutationResolver) UpdateJob(ctx context.Context, jobID int, status model.JobStatus) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

// UpdateTask is the resolver for the updateTask field.
func (r *mutationResolver) UpdateTask(ctx context.Context, taskID int, status model.TaskStatus) (*model.Job, error) {
	panic(fmt.Errorf("not implemented"))
}

// CreateArtifact is the resolver for the createArtifact field.
func (r *mutationResolver) CreateArtifact(ctx context.Context, jobID int, path string, contents string) (*model.Artifact, error) {
	panic(fmt.Errorf("not implemented"))
}

// CreateUserWebhook is the resolver for the createUserWebhook field.
func (r *mutationResolver) CreateUserWebhook(ctx context.Context, config model.UserWebhookInput) (model.WebhookSubscription, error) {
	schema := server.ForContext(ctx).Schema
	if err := corewebhooks.Validate(schema, config.Query); err != nil {
		return nil, err
	}

	user := auth.ForContext(ctx)
	ac, err := corewebhooks.NewAuthConfig(ctx)
	if err != nil {
		return nil, err
	}

	var sub model.UserWebhookSubscription
	if len(config.Events) == 0 {
		return nil, fmt.Errorf("Must specify at least one event")
	}
	events := make([]string, len(config.Events))
	for i, ev := range config.Events {
		events[i] = ev.String()
		// TODO: gqlgen does not support doing anything useful with directives
		// on enums at the time of writing, so we have to do a little bit of
		// manual fuckery
		var access string
		switch ev {
		case model.WebhookEventJobCreated:
			access = "JOBS"
		default:
			return nil, fmt.Errorf("Unsupported event %s", ev.String())
		}
		if !user.Grants.Has(access, auth.RO) {
			return nil, fmt.Errorf("Insufficient access granted for webhook event %s", ev.String())
		}
	}

	u, err := url.Parse(config.URL)
	if err != nil {
		return nil, err
	} else if u.Host == "" {
		return nil, fmt.Errorf("Cannot use URL without host")
	} else if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("Cannot use non-HTTP or HTTPS URL")
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			INSERT INTO gql_user_wh_sub (
				created, events, url, query,
				auth_method,
				token_hash, grants, client_id, expires,
				node_id,
				user_id
			) VALUES (
				NOW() at time zone 'utc',
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
			) RETURNING id, url, query, events, user_id;`,
			pq.Array(events), config.URL, config.Query,
			ac.AuthMethod,
			ac.TokenHash, ac.Grants, ac.ClientID, ac.Expires, // OAUTH2
			ac.NodeID, // INTERNAL
			user.UserID)

		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &sub, nil
}

// DeleteUserWebhook is the resolver for the deleteUserWebhook field.
func (r *mutationResolver) DeleteUserWebhook(ctx context.Context, id int) (model.WebhookSubscription, error) {
	var sub model.UserWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, nil, func(tx *sql.Tx) error {
		row := sq.Delete(`gql_user_wh_sub`).
			PlaceholderFormat(sq.Dollar).
			Where(sq.And{sq.Expr(`id = ?`, id), filter}).
			Suffix(`RETURNING id, url, query, events, user_id`).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(&sub.ID, &sub.URL,
			&sub.Query, pq.Array(&sub.Events), &sub.UserID); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No user webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// DeleteUser is the resolver for the deleteUser field.
func (r *mutationResolver) DeleteUser(ctx context.Context) (int, error) {
	user := auth.ForContext(ctx)
	account.Delete(ctx, user.UserID, user.Username)
	return user.UserID, nil
}

// FromUser is the resolver for the from_user field.
func (r *pGPKeyResolver) FromUser(ctx context.Context, obj *model.PGPKey) (model.Entity, error) {
	if obj.FromUserID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).UsersByID.Load(*obj.FromUserID)
}

// PrivateKey is the resolver for the privateKey field.
func (r *pGPKeyResolver) PrivateKey(ctx context.Context, obj *model.PGPKey) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
	panic(fmt.Errorf("not implemented"))
}

// Version is the resolver for the version field.
func (r *queryResolver) Version(ctx context.Context) (*model.Version, error) {
	conf := config.ForContext(ctx)
	buildTimeout, _ := conf.Get("builds.sr.ht::worker", "timeout")
	sshUser, _ := conf.Get("git.sr.ht::dispatch", "/usr/bin/buildsrht-keys")
	sshUser = strings.Split(sshUser, ":")[0]

	return &model.Version{
		Major:           0,
		Minor:           0,
		Patch:           0,
		DeprecationDate: nil,

		Settings: &model.Settings{
			SSHUser:      sshUser,
			BuildTimeout: buildTimeout,
		},
	}, nil
}

// Me is the resolver for the me field.
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

// UserByID is the resolver for the userByID field.
func (r *queryResolver) UserByID(ctx context.Context, id int) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByID.Load(id)
}

// UserByName is the resolver for the userByName field.
func (r *queryResolver) UserByName(ctx context.Context, username string) (*model.User, error) {
	return loaders.ForContext(ctx).UsersByName.Load(username)
}

// Jobs is the resolver for the jobs field.
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

// Job is the resolver for the job field.
func (r *queryResolver) Job(ctx context.Context, id int) (*model.Job, error) {
	return loaders.ForContext(ctx).JobsByID.Load(id)
}

// Secrets is the resolver for the secrets field.
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

// UserWebhooks is the resolver for the userWebhooks field.
func (r *queryResolver) UserWebhooks(ctx context.Context, cursor *coremodel.Cursor) (*model.WebhookSubscriptionCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	var subs []model.WebhookSubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		sub := (&model.UserWebhookSubscription{}).As(`sub`)
		query := database.
			Select(ctx, sub).
			From(`gql_user_wh_sub sub`).
			Where(filter)
		subs, cursor = sub.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookSubscriptionCursor{subs, cursor}, nil
}

// UserWebhook is the resolver for the userWebhook field.
func (r *queryResolver) UserWebhook(ctx context.Context, id int) (model.WebhookSubscription, error) {
	var sub model.UserWebhookSubscription

	filter, err := corewebhooks.FilterWebhooks(ctx)
	if err != nil {
		return nil, err
	}

	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		row := database.
			Select(ctx, &sub).
			From(`gql_user_wh_sub`).
			Where(sq.And{sq.Expr(`id = ?`, id), filter}).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, &sub)...); err != nil {
			return err
		}
		return nil
	}); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("No user webhook by ID %d found for this user", id)
		}
		return nil, err
	}

	return &sub, nil
}

// Webhook is the resolver for the webhook field.
func (r *queryResolver) Webhook(ctx context.Context) (model.WebhookPayload, error) {
	raw, err := corewebhooks.Payload(ctx)
	if err != nil {
		return nil, err
	}
	payload, ok := raw.(model.WebhookPayload)
	if !ok {
		panic("Invalid webhook payload context")
	}
	return payload, nil
}

// FromUser is the resolver for the from_user field.
func (r *sSHKeyResolver) FromUser(ctx context.Context, obj *model.SSHKey) (model.Entity, error) {
	if obj.FromUserID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).UsersByID.Load(*obj.FromUserID)
}

// PrivateKey is the resolver for the privateKey field.
func (r *sSHKeyResolver) PrivateKey(ctx context.Context, obj *model.SSHKey) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
	panic(fmt.Errorf("not implemented"))
}

// FromUser is the resolver for the from_user field.
func (r *secretFileResolver) FromUser(ctx context.Context, obj *model.SecretFile) (model.Entity, error) {
	if obj.FromUserID == nil {
		return nil, nil
	}
	return loaders.ForContext(ctx).UsersByID.Load(*obj.FromUserID)
}

// Data is the resolver for the data field.
func (r *secretFileResolver) Data(ctx context.Context, obj *model.SecretFile) (string, error) {
	// TODO: This is simple to implement, but I'm not going to rig it up until
	// we need it
	panic(fmt.Errorf("not implemented"))
}

// Log is the resolver for the log field.
func (r *taskResolver) Log(ctx context.Context, obj *model.Task) (*model.Log, error) {
	if obj.Runner == nil {
		return nil, nil
	}
	return FetchLogs(ctx, *obj.Runner, obj.JobID, obj.Name)
}

// Job is the resolver for the job field.
func (r *taskResolver) Job(ctx context.Context, obj *model.Task) (*model.Job, error) {
	return loaders.ForContext(ctx).JobsByID.Load(obj.JobID)
}

// Jobs is the resolver for the jobs field.
func (r *userResolver) Jobs(ctx context.Context, obj *model.User, cursor *coremodel.Cursor) (*model.JobCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	user := auth.ForContext(ctx)
	var jobs []*model.Job
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		job := (&model.Job{}).As(`j`)
		query := database.
			Select(ctx, job).
			From(`job j`).
			Where(sq.And{
				sq.Expr(`j.owner_id = ?`, obj.ID),
				sq.Or{
					sq.Expr(`j.owner_id = ?`, user.UserID),
					sq.Expr(`j.visibility = 'PUBLIC'`),
				},
			})
		jobs, cursor = job.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.JobCursor{jobs, cursor}, nil
}

// Client is the resolver for the client field.
func (r *userWebhookSubscriptionResolver) Client(ctx context.Context, obj *model.UserWebhookSubscription) (*model.OAuthClient, error) {
	if obj.ClientID == nil {
		return nil, nil
	}
	return &model.OAuthClient{
		UUID: *obj.ClientID,
	}, nil
}

// Deliveries is the resolver for the deliveries field.
func (r *userWebhookSubscriptionResolver) Deliveries(ctx context.Context, obj *model.UserWebhookSubscription, cursor *coremodel.Cursor) (*model.WebhookDeliveryCursor, error) {
	if cursor == nil {
		cursor = coremodel.NewCursor(nil)
	}

	var deliveries []*model.WebhookDelivery
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		d := (&model.WebhookDelivery{}).
			WithName(`user`).
			As(`delivery`)
		query := database.
			Select(ctx, d).
			From(`gql_user_wh_delivery delivery`).
			Where(`delivery.subscription_id = ?`, obj.ID)
		deliveries, cursor = d.QueryWithCursor(ctx, tx, query, cursor)
		return nil
	}); err != nil {
		return nil, err
	}

	return &model.WebhookDeliveryCursor{deliveries, cursor}, nil
}

// Sample is the resolver for the sample field.
func (r *userWebhookSubscriptionResolver) Sample(ctx context.Context, obj *model.UserWebhookSubscription, event model.WebhookEvent) (string, error) {
	payloadUUID := uuid.New()
	webhook := corewebhooks.WebhookContext{
		User:        auth.ForContext(ctx),
		PayloadUUID: payloadUUID,
		Name:        "user",
		Event:       event.String(),
		Subscription: &corewebhooks.WebhookSubscription{
			ID:         obj.ID,
			URL:        obj.URL,
			Query:      obj.Query,
			AuthMethod: obj.AuthMethod,
			TokenHash:  obj.TokenHash,
			Grants:     obj.Grants,
			ClientID:   obj.ClientID,
			Expires:    obj.Expires,
			NodeID:     obj.NodeID,
		},
	}

	auth := auth.ForContext(ctx)
	switch event {
	case model.WebhookEventJobCreated:
		note := "Sample job"
		webhook.Payload = &model.JobEvent{
			UUID:  payloadUUID.String(),
			Event: event,
			Date:  time.Now().UTC(),
			Job: &model.Job{
				ID:         -1,
				Created:    time.Now().UTC(),
				Updated:    time.Now().UTC(),
				Manifest:   "image: alpine/latest\ntasks:\n  - hello: echo hello world",
				Note:       &note,
				Image:      "alpine/latest",
				Runner:     nil,
				OwnerID:    auth.UserID,
				JobGroupID: nil,
				RawTags:    nil,
				RawStatus:  "success",
			},
		}
	default:
		return "", fmt.Errorf("Unsupported event %s", event.String())
	}

	subctx := corewebhooks.Context(ctx, webhook.Payload)
	bytes, err := webhook.Exec(subctx, server.ForContext(ctx).Schema)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// Subscription is the resolver for the subscription field.
func (r *webhookDeliveryResolver) Subscription(ctx context.Context, obj *model.WebhookDelivery) (model.WebhookSubscription, error) {
	if obj.Name == "" {
		panic("WebhookDelivery without name")
	}

	// XXX: This could use a loader but it's unlikely to be a bottleneck
	var sub model.WebhookSubscription
	if err := database.WithTx(ctx, &sql.TxOptions{
		Isolation: 0,
		ReadOnly:  true,
	}, func(tx *sql.Tx) error {
		// XXX: This needs some work to generalize to other kinds of webhooks
		var subscription interface {
			model.WebhookSubscription
			database.Model
		} = nil
		switch obj.Name {
		case "user":
			subscription = (&model.UserWebhookSubscription{}).As(`sub`)
		default:
			panic(fmt.Errorf("unknown webhook name %q", obj.Name))
		}
		// Note: No filter needed because, if we have access to the delivery,
		// we also have access to the subscription.
		row := database.
			Select(ctx, subscription).
			From(`gql_`+obj.Name+`_wh_sub sub`).
			Where(`sub.id = ?`, obj.SubscriptionID).
			RunWith(tx).
			QueryRowContext(ctx)
		if err := row.Scan(database.Scan(ctx, subscription)...); err != nil {
			return err
		}
		sub = subscription
		return nil
	}); err != nil {
		return nil, err
	}
	return sub, nil
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

// UserWebhookSubscription returns api.UserWebhookSubscriptionResolver implementation.
func (r *Resolver) UserWebhookSubscription() api.UserWebhookSubscriptionResolver {
	return &userWebhookSubscriptionResolver{r}
}

// WebhookDelivery returns api.WebhookDeliveryResolver implementation.
func (r *Resolver) WebhookDelivery() api.WebhookDeliveryResolver { return &webhookDeliveryResolver{r} }

type jobResolver struct{ *Resolver }
type jobGroupResolver struct{ *Resolver }
type mutationResolver struct{ *Resolver }
type pGPKeyResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type sSHKeyResolver struct{ *Resolver }
type secretFileResolver struct{ *Resolver }
type taskResolver struct{ *Resolver }
type userResolver struct{ *Resolver }
type userWebhookSubscriptionResolver struct{ *Resolver }
type webhookDeliveryResolver struct{ *Resolver }
