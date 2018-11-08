package main

import (
	"database/sql"
	"time"
)

type Job struct {
	db *sql.DB

	Id         int
	Created    time.Time
	Updated    time.Time
	Manifest   string
	OwnerId    int
	JobGroupId *int
	Note       *string
	Status     string
	Runner     *string
	Tags       *string
	Secrets    bool

	Username string
}

type Secret struct {
	Id         int
	UserId     int
	Created    time.Time
	Updated    time.Time
	Uuid       string
	Name       *string
	SecretType string
	Secret     []byte
	Path       *string
	Mode       *int
}

func GetJob(db *sql.DB, id int) (*Job, error) {
	row := db.QueryRow(`
		SELECT
			"job"."id", "job"."created", "job"."updated", "job"."manifest",
			"job"."owner_id", "job"."job_group_id", "job"."note",
			"job"."status", "job"."runner", "job"."tags", "job"."secrets",
			"user".username
		FROM "job"
		JOIN "user" ON "job"."owner_id" = "user"."id"
		WHERE "job"."id" = $1;
	`, id)
	var job Job
	job.db = db
	if err := row.Scan(
		&job.Id, &job.Created, &job.Updated, &job.Manifest, &job.OwnerId,
		&job.JobGroupId, &job.Note, &job.Status, &job.Runner, &job.Tags,
		&job.Secrets, &job.Username); err != nil {

		return nil, err
	}
	return &job, nil
}

func GetSecret(db *sql.DB, uuid string) (*Secret, error) {
	row := db.QueryRow(`
		SELECT
			"id", "user_id", "created", "updated", "uuid",
			"name", "secret_type", "secret", "path", "mode"
		FROM "secret" WHERE "uuid" = $1;
	`, uuid)
	var secret Secret
	if err := row.Scan(
		&secret.Id, &secret.UserId, &secret.Created, &secret.Updated,
		&secret.Uuid, &secret.Name, &secret.SecretType, &secret.Secret,
		&secret.Path, &secret.Mode); err != nil {

		return nil, err
	}
	return &secret, nil
}

func (job *Job) SetRunner(runner string) error {
	_, err := job.db.Exec(`UPDATE "job" SET "runner" = $2 WHERE "id" = $1`,
		job.Id, runner)
	if err == nil {
		_runner := runner
		job.Runner = &_runner
	}
	return err
}

func (job *Job) SetStatus(status string) error {
	_, err := job.db.Exec(`UPDATE "job" SET "status" = $2 WHERE "id" = $1`,
		job.Id, status)
	if err == nil {
		job.Status = status
		_, err = job.db.Exec(`UPDATE "job" SET "updated" = $2 WHERE "id" = $1`,
			job.Id, time.Now().UTC())
	}
	return err
}

func (job *Job) SetTaskStatus(name, status string) error {
	_, err := job.db.Exec(`
		UPDATE "task"
		SET "status" = $3
		WHERE "job_id" = $1 AND "name" = $2
	`, job.Id, name, status)
	return err
}

func (job *Job) GetTaskStatus(name string) (string, error) {
	row := job.db.QueryRow(`
		SELECT "status" FROM "task" WHERE "job_id" = $1 AND "name" = $2
	`, job.Id, name)
	var status string
	if err := row.Scan(&status); err != nil {
		return "", err
	}
	return status, nil
}
