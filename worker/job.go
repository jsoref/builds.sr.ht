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
}

func GetJob(db *sql.DB, id int) (*Job, error) {
	row := db.QueryRow(`
		SELECT
			"id", "created", "updated", "manifest", "owner_id",
			"job_group_id", "note", "status", "runner", "tags",
			"secrets"
		FROM "job" WHERE "id" = $1;
	`, id)
	var job Job
	job.db = db
	if err := row.Scan(
		&job.Id,
		&job.Created,
		&job.Updated,
		&job.Manifest,
		&job.OwnerId,
		&job.JobGroupId,
		&job.Note,
		&job.Status,
		&job.Runner,
		&job.Tags,
		&job.Secrets); err != nil {

		return nil, err
	}
	return &job, nil
}

func (job *Job) SetStatus(status string) error {
	_, err := job.db.Exec(`UPDATE "job" SET "status" = $2 WHERE id = $1`,
		job.Id, status)
	if err != nil {
		return err
	}
	job.Status = status
	return nil
}
