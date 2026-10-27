package httpapi

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/believeboy/mini-bd-program/services/api/internal/audit"
	"github.com/believeboy/mini-bd-program/services/api/internal/security"
)

func recordAudit(
	r *http.Request,
	sink audit.ServiceAPI,
	logger *slog.Logger,
	key []byte,
	eventType string,
	actorType string,
	actorID, shareID *uuid.UUID,
	metadata map[string]any,
) {
	if sink == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		logger.ErrorContext(r.Context(), "encode audit metadata failed",
			"request_id", middleware.GetReqID(r.Context()), "error", err)
		return
	}
	ipHash, userAgentHash := requestHashes(r, key)
	err = sink.Record(r.Context(), audit.Event{
		ActorType: actorType, ActorID: actorID, ShareID: shareID,
		EventType: eventType, RequestID: middleware.GetReqID(r.Context()),
		IPHash: ipHash, UserAgentHash: userAgentHash, Metadata: encoded,
	})
	if err != nil {
		logger.ErrorContext(r.Context(), "record audit event failed",
			"event_type", eventType, "request_id", middleware.GetReqID(r.Context()), "error", err)
	}
}

func requestHashes(r *http.Request, key []byte) ([]byte, []byte) {
	ip := strings.TrimSpace(middleware.GetClientIP(r.Context()))
	if ip == "" {
		ip = strings.TrimSpace(r.RemoteAddr)
	}
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	return security.HMAC(key, ip), security.HMAC(key, r.UserAgent())
}
