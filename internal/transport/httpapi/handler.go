// Package httpapi is the presentation layer: HTTP <-> service translation only.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/LDAi0/phonebook/internal/domain"
	"github.com/LDAi0/phonebook/internal/service"
)

type ContactService interface {
	Create(ctx context.Context, in service.ContactInput) (*domain.Contact, error)
	Get(ctx context.Context, id int64) (*domain.Contact, error)
	List(ctx context.Context, f domain.ListFilter) ([]domain.Contact, int, error)
	Update(ctx context.Context, id int64, in service.ContactInput) (*domain.Contact, error)
	Delete(ctx context.Context, id int64) error
}

type handler struct {
	svc ContactService
	log *slog.Logger
}

type contactRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
}

func (r contactRequest) toInput() service.ContactInput {
	return service.ContactInput{FirstName: r.FirstName, LastName: r.LastName, Phone: r.Phone, Email: r.Email}
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	var req contactRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	c, err := h.svc.Create(r.Context(), req.toInput())
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/contacts/%d", c.ID))
	writeJSON(w, http.StatusCreated, c)
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err := intParam(q.Get("limit"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "limit must be an integer", nil)
		return
	}
	offset, err := intParam(q.Get("offset"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "offset must be an integer", nil)
		return
	}
	items, total, err := h.svc.List(r.Context(), domain.ListFilter{Query: q.Get("q"), Limit: limit, Offset: offset})
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": items, "total": total})
}

func (h *handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req contactRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error(), nil)
		return
	}
	c, err := h.svc.Update(r.Context(), id, req.toInput())
	if err != nil {
		h.handleError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.handleError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	var verr *domain.ValidationError
	switch {
	case errors.As(err, &verr):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "request validation failed", verr.Fields)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrPhoneTaken):
		writeError(w, http.StatusConflict, "phone_taken", err.Error(), nil)
	default:
		h.log.Error("internal error", "path", r.URL.Path, "method", r.Method, "error", err)
		writeError(w, http.StatusInternalServerError, "internal", "internal server error", nil)
	}
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "id must be a positive integer", nil)
		return 0, false
	}
	return id, true
}

func intParam(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	return strconv.Atoi(s)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, msg string, fields map[string]string) {
	writeJSON(w, status, errorBody{Error: errorPayload{Code: code, Message: msg, Fields: fields}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
