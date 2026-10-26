package products

import (
	"bytes"
	"github.com/gofiber/fiber/v2"
	"toolbox/internal/api"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func (h *Handler) Register(router *fiber.App) {
	group := router.Group("/api/v1/products")
	group.Get("", h.List)
	group.Get("/:id", h.Get)
	group.Post("", h.Create)
	group.Put("/:id", h.Update)
	group.Delete("/:id", h.Delete)
	group.All("", func(f *fiber.Ctx) error {
		f.Set("Allow", "GET, POST, PUT, DELETE")
		return f.Status(405).JSON(api.Failure("method_not_allowed", "method not allowed"))
	})
	group.All("/:id", func(f *fiber.Ctx) error {
		f.Set("Allow", "GET, PUT, DELETE")
		return f.Status(405).JSON(api.Failure("method_not_allowed", "method not allowed"))
	})
}
func bad(f *fiber.Ctx, err error) error {
	return f.Status(400).JSON(api.Failure("invalid_request", err.Error()))
}
func fail(f *fiber.Ctx, err error) error {
	status, body := api.ErrorResponse(err)
	return f.Status(status).JSON(body)
}
func (h *Handler) List(f *fiber.Ctx) error {
	limit, offset, err := api.Page(f.Query("limit"), f.Query("offset"))
	if err != nil {
		return bad(f, err)
	}
	records, err := h.service.List(f.UserContext(), limit, offset)
	if err != nil {
		return fail(f, err)
	}
	return f.JSON(api.Success(records))
}
func (h *Handler) Get(f *fiber.Ctx) error {
	id, err := api.ID(f.Params("id"))
	if err != nil {
		return bad(f, err)
	}
	record, err := h.service.Get(f.UserContext(), id)
	if err != nil {
		return fail(f, err)
	}
	return f.JSON(api.Success(record))
}
func (h *Handler) Create(f *fiber.Ctx) error {
	var input Input
	if err := api.Decode(bytes.NewReader(f.Body()), &input); err != nil {
		return bad(f, err)
	}
	if err := input.Validate(); err != nil {
		return bad(f, err)
	}
	record, err := h.service.Create(f.UserContext(), input)
	if err != nil {
		return fail(f, err)
	}
	return f.Status(201).JSON(api.Success(record))
}
func (h *Handler) Update(f *fiber.Ctx) error {
	id, err := api.ID(f.Params("id"))
	if err != nil {
		return bad(f, err)
	}
	var input Input
	if err := api.Decode(bytes.NewReader(f.Body()), &input); err != nil {
		return bad(f, err)
	}
	if err := input.Validate(); err != nil {
		return bad(f, err)
	}
	record, err := h.service.Update(f.UserContext(), id, input)
	if err != nil {
		return fail(f, err)
	}
	return f.JSON(api.Success(record))
}
func (h *Handler) Delete(f *fiber.Ctx) error {
	id, err := api.ID(f.Params("id"))
	if err != nil {
		return bad(f, err)
	}
	if err := h.service.Delete(f.UserContext(), id); err != nil {
		return fail(f, err)
	}
	return f.SendStatus(204)
}
