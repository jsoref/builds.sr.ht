package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"strconv"
	"time"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/database"
	"git.sr.ht/~sircmpwn/core-go/model"
)

type Task struct {
	ID      int        `json:"id"`
	Created time.Time  `json:"created"`
	Updated time.Time  `json:"updated"`
	Name    string     `json:"name"`

	JobID     int
	RawStatus string
	Runner    *string

	alias  string
	fields *database.ModelFields
}

func (t *Task) As(alias string) *Task {
	t.alias = alias
	return t
}

func (t *Task) Alias() string {
	return t.alias
}

func (t *Task) Table() string {
	return `"task"`
}

func (t *Task) Status() TaskStatus {
	st := TaskStatus(strings.ToUpper(t.RawStatus))
	if !st.IsValid() {
		panic(fmt.Errorf("Database invariant broken: invalid status %s for task %d",
			t.RawStatus, t.ID))
	}
	return st
}

func (t *Task) Fields() *database.ModelFields {
	if t.fields != nil {
		return t.fields
	}
	t.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "created", "created", &t.Created },
			{ "updated", "updated", &t.Updated },
			{ "name", "name", &t.Name },
			{ "status", "status", &t.RawStatus },

			// Always fetch:
			{ "id", "", &t.ID },
			{ "job_id", "", &t.JobID },
		},
	}
	return t.fields
}

func (t *Task) QueryWithCursor(ctx context.Context, runner sq.BaseRunner,
	q sq.SelectBuilder, cur *model.Cursor) ([]*Task, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		next, _ := strconv.ParseInt(cur.Next, 10, 64)
		q = q.Where(database.WithAlias(t.alias, "id")+"<= ?", next)
	}
	q = q.
		Join(`job ON job.id = ` + database.WithAlias(t.alias, "job_id")).
		Columns("job.runner").
		OrderBy(database.WithAlias(t.alias, "id") + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		var task Task
		if err := rows.Scan(append(database.Scan(ctx, &task), &task.Runner)...); err != nil {
			panic(err)
		}
		tasks = append(tasks, &task)
	}

	if len(tasks) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.Itoa(tasks[len(tasks)-1].ID),
			Search: cur.Search,
		}
		tasks = tasks[:cur.Count]
	} else {
		cur = nil
	}

	return tasks, cur
}
