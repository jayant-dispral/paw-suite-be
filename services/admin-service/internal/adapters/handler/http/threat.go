package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jayant-dispral/brand-threat-be/shared/domain"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/response"
	"github.com/jayant-dispral/brand-threat-be/shared/ports"
)

// contextKey and UserIDKey are declared in middleware_auth.go (same package).
// They are used here to extract the authenticated user ID from the request context.

type ThreatHandler struct {
	service ports.ThreatService
}

func NewThreatHandler(service ports.ThreatService) *ThreatHandler {
	return &ThreatHandler{service: service}
}

// RegisterRoutes wires all threat endpoints onto the provided chi router.
// Call this from your main router setup:
//
//	r.Route("/projects/{projectID}/threats", func(r chi.Router) {
//	    handler.RegisterRoutes(r)
//	})
func (h *ThreatHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.GetThreatIntel)
	r.Post("/refresh", h.RefreshThreatIntel)
	r.Put("/{threatID}/resolve", h.ResolveThreat)
	r.Put("/{threatID}/ignore", h.IgnoreThreat)
	r.Put("/{threatID}/whitelist", h.WhitelistThreat)
	r.Put("/{threatID}/acknowledge", h.AcknowledgeThreat)
}

func (h *ThreatHandler) GetThreatIntel(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	var (
		data *ports.ThreatIntelResponse
		err  error
	)
	if wantsRefresh(r) {
		data, err = h.service.RefreshThreatIntel(r.Context(), projectID, userID)
	} else {
		data, err = h.service.GetThreatIntel(r.Context(), projectID, userID)
	}
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat intelligence fetched successfully", data)
}

func (h *ThreatHandler) RefreshThreatIntel(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	data, err := h.service.RefreshThreatIntel(r.Context(), projectID, userID)
	if err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat scan refresh queued", data)
}

func wantsRefresh(r *http.Request) bool {
	return queryBool(r, "refresh") || queryBool(r, "force") || queryBool(r, "scan")
}

func queryBool(r *http.Request, key string) bool {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func (h *ThreatHandler) ResolveThreat(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	threatID := chi.URLParam(r, "threatID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	if err := h.service.ResolveThreat(r.Context(), projectID, threatID, userID); err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat resolved successfully", nil)
}

func (h *ThreatHandler) IgnoreThreat(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	threatID := chi.URLParam(r, "threatID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	if err := h.service.IgnoreThreat(r.Context(), projectID, threatID, userID); err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat ignored successfully", nil)
}

// WhitelistThreat permanently suppresses a threat (10-year TTL).
// Use when the lookalike domain is legitimately owned by the brand or a partner.
//
// FIX: new handler wired to the WhitelistThreat service method which was
// added to ports.ThreatService to match the ThreatWhitelisted domain constant.
func (h *ThreatHandler) WhitelistThreat(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	threatID := chi.URLParam(r, "threatID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	if err := h.service.WhitelistThreat(r.Context(), projectID, threatID, userID); err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat whitelisted successfully", nil)
}

// AcknowledgeThreat marks the team as aware without dismissing the threat.
// The threat stays active and re-surfaces on the next scan if still present.
//
// FIX: new handler to expose ThreatAcknowledged, which was already defined in
// domain/threat.go but had no service or HTTP path to reach it.
func (h *ThreatHandler) AcknowledgeThreat(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	threatID := chi.URLParam(r, "threatID")
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok || userID == "" {
		response.WithError(w, domain.ErrUnauthorized)
		return
	}

	if err := h.service.AcknowledgeThreat(r.Context(), projectID, threatID, userID); err != nil {
		response.WithError(w, err)
		return
	}

	response.JSONWithMessage(w, http.StatusOK, "Threat acknowledged successfully", nil)
}
