package http

import (
	"net/http"

	"github.com/mikbil/mikbil/internal/domain"
	"github.com/mikbil/mikbil/internal/usecase"
)

// PackageHandler menangani CRUD paket admin dan katalog public (tech_spec 4.1, 4.4).
type PackageHandler struct {
	pkgs *usecase.PackageUsecase
}

// NewPackageHandler membuat PackageHandler.
func NewPackageHandler(pkgs *usecase.PackageUsecase) *PackageHandler {
	return &PackageHandler{pkgs: pkgs}
}

func packageDTO(p domain.Package) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "description": p.Description,
		"price_idr":      p.PriceIDR,
		"validity_label": p.ValidityLabel, "profile_name": p.ProfileName,
		"rate_limit": p.RateLimit, "session_timeout": p.SessionTimeout,
		"shared_users": p.SharedUsers, "is_active": p.IsActive,
		"sort_order": p.SortOrder,
	}
}

func publicPackageDTO(p domain.Package) map[string]any {
	return map[string]any{
		"id": p.ID, "name": p.Name, "description": p.Description,
		"price_idr": p.PriceIDR, "validity_label": p.ValidityLabel,
		"profile_name": p.ProfileName, "rate_limit": p.RateLimit,
	}
}

// ListPublic menangani GET /api/packages (hanya paket aktif).
func (h *PackageHandler) ListPublic(w http.ResponseWriter, _ *http.Request) {
	ps, err := h.pkgs.ListActive()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, publicPackageDTO(p))
	}
	WriteOK(w, http.StatusOK, out)
}

// List menangani GET /api/admin/packages.
func (h *PackageHandler) List(w http.ResponseWriter, _ *http.Request) {
	ps, err := h.pkgs.List()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, packageDTO(p))
	}
	WriteOK(w, http.StatusOK, out)
}

type packageBody struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	PriceIDR       int    `json:"price_idr"`
	ValidityLabel  string `json:"validity_label"`
	ProfileName    string `json:"profile_name"`
	RateLimit      string `json:"rate_limit"`
	SessionTimeout int    `json:"session_timeout"`
	SharedUsers    int    `json:"shared_users"`
	IsActive       *bool  `json:"is_active"`
	SortOrder      int    `json:"sort_order"`
	CreateProfile  bool   `json:"create_profile"`
}

func (b packageBody) input() usecase.PackageInput {
	active := true
	if b.IsActive != nil {
		active = *b.IsActive
	}
	return usecase.PackageInput{
		Name: b.Name, Description: b.Description, PriceIDR: b.PriceIDR,
		ValidityLabel: b.ValidityLabel,
		ProfileName:   b.ProfileName, RateLimit: b.RateLimit,
		SessionTimeout: b.SessionTimeout, SharedUsers: b.SharedUsers,
		IsActive: active, SortOrder: b.SortOrder, CreateProfile: b.CreateProfile,
	}
}

// Create menangani POST /api/admin/packages.
func (h *PackageHandler) Create(w http.ResponseWriter, r *http.Request) {
	var b packageBody
	if !decodeJSON(w, r, &b) {
		return
	}
	p, err := h.pkgs.Create(b.input())
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusCreated, packageDTO(p))
}

// Update menangani PUT /api/admin/packages/:id.
func (h *PackageHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var b packageBody
	if !decodeJSON(w, r, &b) {
		return
	}
	p, err := h.pkgs.Update(id, b.input())
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, packageDTO(p))
}

// Delete menangani DELETE /api/admin/packages/:id.
func (h *PackageHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.pkgs.Delete(id); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"deleted": true})
}
