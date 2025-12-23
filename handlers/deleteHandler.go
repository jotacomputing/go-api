package handlers

import (
	"jotacomputing/go-api/queue"
	"jotacomputing/go-api/structs"
	"net/http"
	"strconv"

	echoserver "github.com/dasjott/oauth2-echo-server"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/labstack/echo/v4"
)

// it will construct the order and add it to the Cancel order queue
func CancelOrderHandler(c echo.Context) error {
	// Get authenticated user from OAuth2 token
	ti, exists := c.Get(echoserver.DefaultConfig.TokenKey).(oauth2.TokenInfo)
	if !exists {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid or missing token")
	}

	// Parse string userID back to uint64 (matches your matching engine)
	userIDStr := ti.GetUserID()
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Invalid user ID format")
	}

	var tempOrderCancel structs.TempOrderToBeCancelled
	if err := c.Bind(&tempOrderCancel); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request body"})
	}

	// Create order cancel with AUTHENTICATED user_id (secure - from token, not request!)
	var cancelOrder structs.OrderToBeCancelled
	cancelOrder.Order_id = tempOrderCancel.Order_id
	cancelOrder.User_id = userID
	cancelOrder.Symbol = tempOrderCancel.Symbol

	// Enqueue the order cancel
	if err := queue.CancelOrderQueue.Enqueue(cancelOrder); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enqueue cancel order")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "Order cancel request sent successfully",
		"order_id": cancelOrder.Order_id,
		"user_id":  userID,
		"symbol":   cancelOrder.Symbol,
	})
}
