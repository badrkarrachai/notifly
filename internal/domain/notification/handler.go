package notification

import (
	"log/slog"
	"net/http"

	"notifly/internal/common"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the notification domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new notification handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Send handles POST /api/v1/send
// Enqueues a notification for async processing and returns 202 Accepted.
func (h *Handler) Send(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	resp, err := h.service.Enqueue(c.Request.Context(), &req)
	if err != nil {
		slog.Error("enqueue notification failed",
			"error", err,
			"channel", req.Channel,
			"type", req.Type,
			"to", req.To,
		)
		common.HandleError(c, err)
		return
	}

	common.Success(c, http.StatusAccepted, resp)
}

// GetNotification handles GET /api/v1/notifications/:id
func (h *Handler) GetNotification(c *gin.Context) {
	id := c.Param("id")

	notifLog, err := h.service.GetNotification(c.Request.Context(), id)
	if err != nil {
		common.HandleError(c, err)
		return
	}

	common.Success(c, http.StatusOK, notifLog)
}

// ListNotifications handles GET /api/v1/notifications
func (h *Handler) ListNotifications(c *gin.Context) {
	var filter ListFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		common.Error(c, http.StatusBadRequest, "invalid query parameters: "+err.Error())
		return
	}

	resp, err := h.service.ListNotifications(c.Request.Context(), filter)
	if err != nil {
		common.HandleError(c, err)
		return
	}

	common.Success(c, http.StatusOK, resp)
}

// ResendWebhook handles POST /api/v1/webhooks/resend
// Receives delivery status updates from Resend webhooks.
func (h *Handler) ResendWebhook(c *gin.Context) {
	var event struct {
		Type string `json:"type"`
		Data struct {
			EmailID string `json:"email_id"`
		} `json:"data"`
	}

	if err := c.ShouldBindJSON(&event); err != nil {
		common.Error(c, http.StatusBadRequest, "invalid webhook payload: "+err.Error())
		return
	}

	// Map Resend event types to our notification statuses
	var status NotificationStatus
	switch event.Type {
	case "email.delivered":
		status = StatusDelivered
	case "email.bounced":
		status = StatusBounced
	case "email.opened":
		status = StatusOpened
	default:
		// Acknowledge but ignore unhandled event types
		slog.Info("ignoring webhook event", "type", event.Type)
		common.Success(c, http.StatusOK, gin.H{"status": "ignored"})
		return
	}

	if err := h.service.HandleWebhookEvent(c.Request.Context(), event.Data.EmailID, status); err != nil {
		slog.Error("webhook processing failed",
			"event_type", event.Type,
			"email_id", event.Data.EmailID,
			"error", err,
		)
		common.HandleError(c, err)
		return
	}

	common.Success(c, http.StatusOK, gin.H{"status": "processed"})
}

// RegisterRoutes registers notification routes to the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/send", h.Send)
	rg.GET("/notifications", h.ListNotifications)
	rg.GET("/notifications/:id", h.GetNotification)
	rg.POST("/webhooks/resend", h.ResendWebhook)
}
