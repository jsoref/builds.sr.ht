package loaders

//go:generate ./gen UsersByIDLoader int api/graph/model.User
//go:generate ./gen UsersByNameLoader string api/graph/model.User
//go:generate ./gen JobsByIDLoader int api/graph/model.Job
//go:generate ./gen JobGroupsByIDLoader int api/graph/model.JobGroup

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"

	"git.sr.ht/~sircmpwn/builds.sr.ht/api/graph/model"
	"git.sr.ht/~sircmpwn/core-go/database"
)

var loadersCtxKey = &contextKey{"loaders"}

type contextKey struct {
	name string
}

type Loaders struct {
	UsersByID     UsersByIDLoader
	UsersByName   UsersByNameLoader
	JobsByID      JobsByIDLoader
	JobGroupsByID JobGroupsByIDLoader
}

func fetchUsersByID(ctx context.Context) func(ids []int) ([]*model.User, []error) {
	return func(ids []int) ([]*model.User, []error) {
		users := make([]*model.User, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.User{}).As(`u`)).
				From(`"user" u`).
				Where(sq.Expr(`u.id = ANY(?)`, pq.Array(ids)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				return err
			}
			defer rows.Close()

			usersByID := map[int]*model.User{}
			for rows.Next() {
				user := model.User{}
				if err := rows.Scan(database.Scan(ctx, &user)...); err != nil {
					return err
				}
				usersByID[user.ID] = &user
			}
			if err = rows.Err(); err != nil {
				return err
			}

			for i, id := range ids {
				users[i] = usersByID[id]
			}

			return nil
		}); err != nil {
			panic(err)
		}

		return users, nil
	}
}

func fetchUsersByName(ctx context.Context) func(names []string) ([]*model.User, []error) {
	return func(names []string) ([]*model.User, []error) {
		users := make([]*model.User, len(names))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.User{}).As(`u`)).
				From(`"user" u`).
				Where(sq.Expr(`u.username = ANY(?)`, pq.Array(names)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				return err
			}
			defer rows.Close()

			usersByName := map[string]*model.User{}
			for rows.Next() {
				user := model.User{}
				if err := rows.Scan(database.Scan(ctx, &user)...); err != nil {
					return err
				}
				usersByName[user.Username] = &user
			}
			if err = rows.Err(); err != nil {
				return err
			}

			for i, name := range names {
				users[i] = usersByName[name]
			}

			return nil
		}); err != nil {
			panic(err)
		}

		return users, nil
	}
}

func fetchJobsByID(ctx context.Context) func(ids []int) ([]*model.Job, []error) {
	return func(ids []int) ([]*model.Job, []error) {
		jobs := make([]*model.Job, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.Job{})).
				From(`job`).
				Where(sq.Expr(`job.id = ANY(?)`, pq.Array(ids)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				return err
			}
			defer rows.Close()

			jobsByID := map[int]*model.Job{}
			for rows.Next() {
				job := model.Job{}
				if err := rows.Scan(database.Scan(ctx, &job)...); err != nil {
					return err
				}
				jobsByID[job.ID] = &job
			}
			if err = rows.Err(); err != nil {
				return err
			}

			for i, id := range ids {
				jobs[i] = jobsByID[id]
			}

			return nil
		}); err != nil {
			panic(err)
		}

		return jobs, nil
	}
}

func fetchJobGroupsByID(ctx context.Context) func(ids []int) ([]*model.JobGroup, []error) {
	return func(ids []int) ([]*model.JobGroup, []error) {
		groups := make([]*model.JobGroup, len(ids))
		if err := database.WithTx(ctx, &sql.TxOptions{
			Isolation: 0,
			ReadOnly:  true,
		}, func(tx *sql.Tx) error {
			var (
				err  error
				rows *sql.Rows
			)
			query := database.
				Select(ctx, (&model.JobGroup{})).
				From(`job_group`).
				Where(sq.Expr(`job_group.id = ANY(?)`, pq.Array(ids)))
			if rows, err = query.RunWith(tx).QueryContext(ctx); err != nil {
				return err
			}
			defer rows.Close()

			groupsByID := map[int]*model.JobGroup{}
			for rows.Next() {
				group := model.JobGroup{}
				if err := rows.Scan(database.Scan(ctx, &group)...); err != nil {
					return err
				}
				groupsByID[group.ID] = &group
			}
			if err = rows.Err(); err != nil {
				return err
			}

			for i, id := range ids {
				groups[i] = groupsByID[id]
			}

			return nil
		}); err != nil {
			panic(err)
		}

		return groups, nil
	}
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), loadersCtxKey, &Loaders{
			UsersByID: UsersByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchUsersByID(r.Context()),
			},
			UsersByName: UsersByNameLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchUsersByName(r.Context()),
			},
			JobsByID: JobsByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchJobsByID(r.Context()),
			},
			JobGroupsByID: JobGroupsByIDLoader{
				maxBatch: 100,
				wait:     1 * time.Millisecond,
				fetch:    fetchJobGroupsByID(r.Context()),
			},
		})
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func ForContext(ctx context.Context) *Loaders {
	raw, ok := ctx.Value(loadersCtxKey).(*Loaders)
	if !ok {
		panic(errors.New("Invalid data loaders context"))
	}
	return raw
}
