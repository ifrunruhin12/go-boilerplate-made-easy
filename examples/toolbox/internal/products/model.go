package products

import (
	"fmt"
	"math"
	"toolbox/internal/api"
)

type Record struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int64   `json:"quantity"`
	Active   bool    `json:"active"`
}
type Input struct {
	Name     *string  `json:"name"`
	Price    *float64 `json:"price"`
	Quantity *int64   `json:"quantity"`
	Active   *bool    `json:"active"`
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func (v Input) Validate() error {
	if v.Name == nil {
		return fmt.Errorf("name is required")
	}
	if err := api.ValidateString("name", *v.Name); err != nil {
		return err
	}
	if v.Price == nil {
		return fmt.Errorf("price is required")
	}
	if !finite(*v.Price) {
		return fmt.Errorf("price must be finite")
	}
	if v.Quantity == nil {
		return fmt.Errorf("quantity is required")
	}
	if v.Active == nil {
		return fmt.Errorf("active is required")
	}
	return nil
}
func (v Input) Record() Record {
	return Record{Name: *v.Name, Price: *v.Price, Quantity: *v.Quantity, Active: *v.Active}
}
