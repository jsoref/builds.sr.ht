package model

import (
	"context"
	"database/sql"
	"time"
	"strconv"

	sq "github.com/Masterminds/squirrel"

	"git.sr.ht/~sircmpwn/core-go/model"
	"git.sr.ht/~sircmpwn/core-go/database"
)

const (
	SECRET_PGPKEY = "pgp_key"
	SECRET_SSHKEY = "ssh_key"
	SECRET_FILE = "plaintext_file"
)

type Secret interface {
	IsSecret()
}

type RawSecret struct {
	ID         int
	Created    time.Time
	UUID       string
	SecretType string
	Secret     []byte
	Name       *string
	Path       *string
	Mode       *int

	alias  string
	fields *database.ModelFields
}

func (s *RawSecret) As(alias string) *RawSecret {
	s.alias = alias
	return s
}

func (s *RawSecret) Alias() string {
	return s.alias
}

func (s *RawSecret) Table() string {
	return `"secret"`
}

type PGPKey struct {
	ID         int       `json:"id"`
	Created    time.Time `json:"created"`
	UUID       string    `json:"uuid"`
	Name       *string   `json:"name"`
	PrivateKey []byte    `json:"privateKey"`
}

func (PGPKey) IsSecret() {}

type SSHKey struct {
	ID         int       `json:"id"`
	Created    time.Time `json:"created"`
	UUID       string    `json:"uuid"`
	Name       *string   `json:"name"`
	PrivateKey []byte    `json:"privateKey"`
}

func (SSHKey) IsSecret() {}

type SecretFile struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`
	UUID    string    `json:"uuid"`
	Name    *string   `json:"name"`
	Path    string    `json:"path"`
	Mode    int       `json:"mode"`
	Data    []byte    `json:"data"`
}

func (SecretFile) IsSecret() {}

func (s *RawSecret) ToSecret() Secret {
	switch s.SecretType {
	case SECRET_PGPKEY:
		return &PGPKey{
			ID: s.ID,
			Created: s.Created,
			UUID: s.UUID,
			Name: s.Name,
			PrivateKey: s.Secret,
		}
	case SECRET_SSHKEY:
		return &SSHKey{
			ID: s.ID,
			Created: s.Created,
			UUID: s.UUID,
			Name: s.Name,
			PrivateKey: s.Secret,
		}
	case SECRET_FILE:
		return &SecretFile{
			ID: s.ID,
			Created: s.Created,
			UUID: s.UUID,
			Name: s.Name,
			Path: *s.Path,
			Mode: *s.Mode,
			Data: s.Secret,
		}
	default:
		panic("Database invariant broken: unknown secret type")
	}
}

func (s *RawSecret) Fields() *database.ModelFields {
	if s.fields != nil {
		return s.fields
	}
	s.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "created", "created", &s.Created },
			{ "uuid", "uuid", &s.UUID },
			{ "name", "name", &s.Name },

			// Always fetch:
			{ "id", "", &s.ID },
			{ "secret_type", "", &s.SecretType },
			{ "secret", "", &s.Secret },
			{ "path", "", &s.Path },
			{ "mode", "", &s.Mode },
		},
	}
	return s.fields
}

func (s *RawSecret) QueryWithCursor(ctx context.Context, runner sq.BaseRunner,
	q sq.SelectBuilder, cur *model.Cursor) ([]Secret, *model.Cursor) {
	var (
		err  error
		rows *sql.Rows
	)

	if cur.Next != "" {
		next, _ := strconv.ParseInt(cur.Next, 10, 64)
		q = q.Where(database.WithAlias(s.alias, "id")+"<= ?", next)
	}
	q = q.
		OrderBy(database.WithAlias(s.alias, "id") + " DESC").
		Limit(uint64(cur.Count + 1))

	if rows, err = q.RunWith(runner).QueryContext(ctx); err != nil {
		panic(err)
	}
	defer rows.Close()

	var (
		secrets []Secret
		lastId  int
	)
	for rows.Next() {
		var secret RawSecret
		if err := rows.Scan(database.Scan(ctx, &secret)...); err != nil {
			panic(err)
		}
		lastId = secret.ID
		secrets = append(secrets, secret.ToSecret())
	}

	if len(secrets) > cur.Count {
		cur = &model.Cursor{
			Count:  cur.Count,
			Next:   strconv.Itoa(lastId),
			Search: cur.Search,
		}
		secrets = secrets[:cur.Count]
	} else {
		cur = nil
	}

	return secrets, cur
}
