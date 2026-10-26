package users

import (
	"fmt"
	"toolbox/internal/api"
)

type Record struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
type Input struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

func (v Input) Validate() error {
	if v.Name == nil {
		return fmt.Errorf("name is required")
	}
	if err := api.ValidateString("name", *v.Name); err != nil {
		return err
	}
	if v.Email == nil {
		return fmt.Errorf("email is required")
	}
	if err := api.ValidateString("email", *v.Email); err != nil {
		return err
	}
	return nil
}
func (v Input) Record() Record { return Record{Name: *v.Name, Email: *v.Email} }
