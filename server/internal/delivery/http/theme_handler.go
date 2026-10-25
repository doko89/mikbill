package http

import (
	"io"
	"net/http"
	"strings"

	"github.com/mikbil/mikbil/internal/usecase"
)

// Batas body upload logo (512KB + ruang overhead multipart).
const maxLogoBodyBytes = 768 << 10

// ThemeHandler menangani tema publik dan logo (kontrak tema v1).
type ThemeHandler struct {
	theme *usecase.ThemeUsecase
}

// NewThemeHandler membuat ThemeHandler.
func NewThemeHandler(theme *usecase.ThemeUsecase) *ThemeHandler {
	return &ThemeHandler{theme: theme}
}

type themeDTO struct {
	BrandName    string  `json:"brand_name"`
	BrandTagline string  `json:"brand_tagline"`
	AccentColor  string  `json:"accent_color"`
	ThemeMode    string  `json:"theme_mode"`
	LogoURL      *string `json:"logo_url"`
}

func toThemeDTO(v usecase.ThemeView) themeDTO {
	d := themeDTO{
		BrandName:    v.BrandName,
		BrandTagline: v.BrandTagline,
		AccentColor:  v.AccentColor,
		ThemeMode:    v.ThemeMode,
	}
	if v.LogoVersion != "" {
		u := "/api/theme/logo?v=" + v.LogoVersion
		d.LogoURL = &u
	}
	return d
}

// Get menangani GET /api/theme (publik).
func (h *ThemeHandler) Get(w http.ResponseWriter, r *http.Request) {
	v, err := h.theme.Get()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, toThemeDTO(v))
}

// Logo menangani GET /api/theme/logo (publik, cache abadi per versi).
func (h *ThemeHandler) Logo(w http.ResponseWriter, r *http.Request) {
	v, found, err := h.theme.GetLogo()
	if err != nil {
		writeUsecaseErr(w, err)
		return
	}
	if !found {
		WriteErr(w, http.StatusNotFound, "logo belum dipasang")
		return
	}
	etag := `"` + v.LogoVersion + `"`
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", v.LogoMIME)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(v.LogoData)
}

// UploadLogo menangani PUT /api/admin/theme/logo (multipart field logo).
func (h *ThemeHandler) UploadLogo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBodyBytes)
	f, _, err := r.FormFile("logo")
	if err != nil {
		WriteErr(w, http.StatusBadRequest, "field logo wajib diisi")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxLogoBodyBytes))
	if err != nil {
		WriteErr(w, http.StatusBadRequest, "gagal membaca berkas logo")
		return
	}
	mime := sniffImageMIME(data)
	v, uerr := h.theme.SaveLogo(mime, data)
	if uerr != nil {
		writeUsecaseErr(w, uerr)
		return
	}
	u := "/api/theme/logo?v=" + v.LogoVersion
	WriteOK(w, http.StatusOK, map[string]string{"logo_url": u})
}

// DeleteLogo menangani DELETE /api/admin/theme/logo.
func (h *ThemeHandler) DeleteLogo(w http.ResponseWriter, r *http.Request) {
	if err := h.theme.DeleteLogo(); err != nil {
		writeUsecaseErr(w, err)
		return
	}
	WriteOK(w, http.StatusOK, map[string]bool{"deleted": true})
}

// sniffImageMIME menebak tipe berkas dari bytes; hanya png/jpeg yang lolos validasi.
func sniffImageMIME(data []byte) string {
	ct := http.DetectContentType(data)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct
}
