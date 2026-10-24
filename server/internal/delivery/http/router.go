package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mikbil/mikbil/internal/delivery/web"
	"github.com/mikbil/mikbil/internal/usecase"
)

// Deps adalah dependensi router HTTP.
type Deps struct {
	Setup  *usecase.SetupUsecase
	Auth   *usecase.AuthUsecase
	Conn   *usecase.ConnectionUsecase
	Prof   *usecase.ProfileUsecase
	Prov   *usecase.ProvisioningUsecase
	Hot    *usecase.HotspotUsecase
	Allow  *usecase.AllowlistUsecase
	Pkgs   *usecase.PackageUsecase
	Ord    *usecase.OrderUsecase
	Pay    *usecase.PaymentUsecase
	Adm    *usecase.AdminOrderUsecase
	Health *usecase.HealthUsecase
	Set    *usecase.SettingsUsecase
}

// NewRouter menyusun semua route API dan file statis frontend.
func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	setup := NewSetupHandler(d.Setup)
	admin := NewAdminHandler(d.Auth)
	router := NewRouterHandler(d.Conn, d.Prof, d.Prov, d.Hot, d.Allow)
	pkgs := NewPackageHandler(d.Pkgs)
	ord := NewOrderHandler(d.Ord)
	hook := NewWebhookHandler(d.Pay)
	adm := NewAdminOrderHandler(d.Adm)
	set := NewSettingsHandler(d.Set)
	lim := newRateLimiter()

	r.Route("/api", func(r chi.Router) {
		// Status setup tetap aktif setelah finish (untuk guard first-run).
		r.Get("/setup/status", setup.Status)
		if d.Health != nil {
			r.Get("/health", NewHealthHandler(d.Health).Get)
		}
		r.Get("/packages", pkgs.ListPublic)
		r.With(lim.limit(10, rateWindow, ipKey)).Post("/orders", ord.Create)
		r.With(lim.limit(30, rateWindow, ipPathKey)).Get("/orders/{id}", ord.Get)
		r.Post("/orders/{id}/cancel", ord.Cancel)
		r.With(lim.limit(120, rateWindow, ipKey)).Post("/webhooks/kriskris", hook.KrisKris)
		r.Route("/setup", func(r chi.Router) {
			r.Use(setup.Guard)
			r.Post("/admin", setup.CreateAdmin)
			r.Post("/site", setup.SaveSite)
			r.Post("/connection", setup.SaveConnection)
			r.Post("/connection/test", setup.TestConnection)
			r.Post("/kriskris", setup.SaveKrisKris)
			r.Post("/provision", setup.Provision)
			r.Post("/finish", setup.Finish)
		})
		r.With(lim.limit(5, rateWindow, ipKey)).Post("/admin/login", admin.Login)
		r.Route("/admin", func(r chi.Router) {
			r.Use(RequireAuth(d.Auth))
			r.Post("/logout", admin.Logout)
			r.Get("/me", admin.Me)
			r.Get("/connections", router.ListConnections)
			r.Post("/connections", router.CreateConnection)
			r.Put("/connections/{id}", router.UpdateConnection)
			r.Delete("/connections/{id}", router.DeleteConnection)
			r.Post("/connections/{id}/test", router.TestConnection)
			r.Get("/router/profiles", router.ListProfiles)
			r.Post("/router/profiles", router.CreateProfile)
			r.Get("/provisioning/status", router.ProvisioningStatus)
			r.Post("/provisioning/apply", router.ProvisioningApply)
			r.Get("/walled-garden", router.ListAllowlist)
			r.Post("/walled-garden", router.CreateAllowlist)
			r.Post("/walled-garden/bulk", router.CreateAllowlistBulk)
			r.Put("/walled-garden/{id}", router.UpdateAllowlist)
			r.Delete("/walled-garden/{id}", router.DeleteAllowlist)
			r.Get("/hotspot/login-template", router.LoginTemplate)
			r.Get("/hotspot/button-snippet", router.ButtonSnippet)
			r.Get("/packages", pkgs.List)
			r.Post("/packages", pkgs.Create)
			r.Put("/packages/{id}", pkgs.Update)
			r.Delete("/packages/{id}", pkgs.Delete)
			r.Get("/settings", set.Get)
			r.Put("/settings", set.Update)
			r.Get("/orders/summary", adm.Summary)
			r.Get("/orders", adm.List)
			r.Get("/orders/{id}", adm.Get)
			r.Post("/orders/{id}/retry", adm.Retry)
		})
	})

	notFound := func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			WriteErr(w, http.StatusNotFound, "tidak ditemukan")
			return
		}
		web.Handler().ServeHTTP(w, req)
	}
	r.NotFound(notFound)
	r.MethodNotAllowed(func(w http.ResponseWriter, req *http.Request) {
		WriteErr(w, http.StatusMethodNotAllowed, "metode tidak diizinkan")
	})

	// Catch-all GET untuk SPA; unknown /api/* tetap 404 berenvelope.
	r.Get("/*", notFound)
	return r
}
