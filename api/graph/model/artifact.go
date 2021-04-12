package model

import (
	"time"

	"git.sr.ht/~sircmpwn/core-go/database"
)

type Artifact struct {
	ID      int       `json:"id"`
	Created time.Time `json:"created"`
	Path    string    `json:"path"`
	Size    int       `json:"size"`
	URL     *string   `json:"url"`

	alias  string
	fields *database.ModelFields
}

func (a *Artifact) As(alias string) *Artifact {
	a.alias = alias
	return a
}

func (a *Artifact) Alias() string {
	return a.alias
}

func (a *Artifact) Table() string {
	return `"artifact"`
}

func (a *Artifact) Fields() *database.ModelFields {
	if a.fields != nil {
		return a.fields
	}
	a.fields = &database.ModelFields{
		Fields: []*database.FieldMap{
			{ "created", "created", &a.Created },
			{ "path", "path", &a.Path },
			{ "size", "size", &a.Size },
			{ "url", "url", &a.URL },

			// Always fetch:
			{ "id", "", &a.ID },
		},
	}
	return a.fields
}
