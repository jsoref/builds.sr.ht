package model

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

type Job struct {
	ID        int       `json:"id"`
	Created   time.Time `json:"created"`
	Updated   time.Time `json:"updated"`
	Manifest  string    `json:"manifest"`
	Note      *string   `json:"note"`
	Image     string    `json:"image"`
	Runner    *string   `json:"runner"`

	OwnerID    int
	JobGroupID *int
	RawTags    *string
	RawStatus  string

	alias  string
	fields *database.ModelFields
}

func (j *Job) As(alias string) *Job {
	j.alias = alias
	return j
}

func (j *Job) Alias() string {
	return j.alias
}

func (j *Job) Table() string {
	return `"job"`
}

func (j *Job) Tags() []string {
	// TODO: Store me as a pg array
	if j.RawTags == nil {
		return nil
	}
	return strings.Split(*j.RawTags, "/")
}

func (j *Job) Status() JobStatus {
	st := JobStatus(strings.ToUpper(j.RawStatus))
	if !st.IsValid() {
		panic(fmt.Errorf("Database invariant broken: invalid status %s for job %d",
			j.RawStatus, j.ID))
	}
	return st
}

func (j *Job) Fields() *database.ModelFields {
	if j.fields != nil {
		return j.fields
	}
	j.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "created", "created", &j.Created },
			{ "updated", "updated", &j.Updated },
			{ "manifest", "manifest", &j.Manifest },
			{ "note", "note", &j.Note },
			{ "tags", "tags", &j.RawTags },
			{ "status", "status", &j.RawStatus },
			{ "image", "image", &j.Image },

			// Always fetch:
			{ "id", "", &j.ID },
			{ "owner_id", "", &j.OwnerID },
			{ "job_group_id", "", &j.JobGroupID },
			{ "runner", "", &j.Runner },
		},
	}
	return j.fields
}

func (j *Job) QueryWithCursor(ctx context.Context, runner sq.BaseRunner,
	q sq.SelectBuilder, cur *model.Cursor) ([]*Job, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		next, _ := strconv.ParseInt(cur.Next, 10, 64)
		q = q.Where(database.WithAlias(j.alias, "id")+"<= ?", next)
	}
	q = q.
		OrderBy(database.WithAlias(j.alias, "id") + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		var job Job
		if err := rows.Scan(database.Scan(ctx, &job)...); err != nil {
			panic(err)
		}
		jobs = append(jobs, &job)
	}

	if len(jobs) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.Itoa(jobs[len(jobs)-1].ID),
			Search: cur.Search,
		}
		jobs = jobs[:cur.Count]
	} else {
		cur = nil
	}

	return jobs, cur
}
