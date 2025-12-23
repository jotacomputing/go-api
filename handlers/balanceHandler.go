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

func GetBalanceHandler(c echo.Context) error {
	// it juist sends thw query to the balance manager
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
	var tempQuery structs.TempQuery
	if err := c.Bind(&tempQuery); err != nil {
		return c.JSON(400, map[string]string{"error": "Invalid request body"})
	}
	var query structs.Query
	query.Query_id = tempQuery.Query_id
	query.Query_type = 0 // get balance
	query.User_id = userID

	// Enqueue the query
	if err := queue.QueriesQueue.Enqueue(query); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enqueue balance query")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":   "Balance query sent successfully",
		"query_id": query.Query_id,
		"user_id":  userID,
	})
}
