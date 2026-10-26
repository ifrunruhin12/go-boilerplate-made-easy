package users

import (
	"minimal/internal/api"
	"net/http"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service} }
func (h *Handler) Register(router *http.ServeMux) {
	router.HandleFunc("GET /api/v1/users", h.List)
	router.HandleFunc("POST /api/v1/users", h.Create)
	router.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET, POST")
		api.Write(w, 405, api.Failure("method_not_allowed", "method not allowed"))
	})
}
func bad(w http.ResponseWriter, err error) {
	api.Write(w, 400, api.Failure("invalid_request", err.Error()))
}
func fail(w http.ResponseWriter, err error) {
	status, body := api.ErrorResponse(err)
	api.Write(w, status, body)
}
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := api.Page(r.URL.Query().Get("limit"), r.URL.Query().Get("offset"))
	if err != nil {
		bad(w, err)
		return
	}
	records, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		fail(w, err)
		return
	}
	api.Write(w, 200, api.Success(records))
}
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := api.ID(r.PathValue("id"))
	if err != nil {
		bad(w, err)
		return
	}
	record, err := h.service.Get(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	api.Write(w, 200, api.Success(record))
}
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var input Input
	if err := api.Decode(r.Body, &input); err != nil {
		bad(w, err)
		return
	}
	if err := input.Validate(); err != nil {
		bad(w, err)
		return
	}
	record, err := h.service.Create(r.Context(), input)
	if err != nil {
		fail(w, err)
		return
	}
	api.Write(w, 201, api.Success(record))
}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := api.ID(r.PathValue("id"))
	if err != nil {
		bad(w, err)
		return
	}
	var input Input
	if err := api.Decode(r.Body, &input); err != nil {
		bad(w, err)
		return
	}
	if err := input.Validate(); err != nil {
		bad(w, err)
		return
	}
	record, err := h.service.Update(r.Context(), id, input)
	if err != nil {
		fail(w, err)
		return
	}
	api.Write(w, 200, api.Success(record))
}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := api.ID(r.PathValue("id"))
	if err != nil {
		bad(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
