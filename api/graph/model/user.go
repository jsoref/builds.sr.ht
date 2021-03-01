package model

import (
	"fmt"
	"time"
)

type User struct {
	ID       int        `json:"id"`
	Created  time.Time  `json:"created"`
	Updated  time.Time  `json:"updated"`
	Username string     `json:"username"`
	Email    string     `json:"email"`
	URL      *string    `json:"url"`
	Location *string    `json:"location"`
	Bio      *string    `json:"bio"`
}

func (User) IsEntity() {}

func (u User) CanonicalName() string {
	return fmt.Sprintf("~%s", u.Username)
}
