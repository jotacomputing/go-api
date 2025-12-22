package handlers

import (
    "jotacomputing/go-api/structs"
    "net/http"
    "jotacomputing/go-api/queue"
    "jotacomputing/go-api/utils"
    "strconv"
    
    "github.com/labstack/echo/v4"
    "github.com/go-oauth2/oauth2/v4"
    echoserver "github.com/dasjott/oauth2-echo-server"
)

func PostOrderHandler(c echo.Context) error {
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

    var tempOrder structs.TempOrder
    if err := c.Bind(&tempOrder); err != nil {
        return c.JSON(400, map[string]string{"error": "Invalid request body"})
    }
    
    // Validate order fields
    if err := tempOrder.Validate(); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, err.Error())
    }

    // Create order with AUTHENTICATED user_id (secure - from token, not request!)
    var order structs.Order
    order.Order_id = tempOrder.Order_id
    order.Price = tempOrder.Price
    order.Timestamp = tempOrder.Timestamp
    order.User_id = userID       
    order.Shares_qty = tempOrder.Shares_qty
    order.Symbol = tempOrder.Symbol
    order.Side = tempOrder.Side
    order.Order_type = tempOrder.Order_type
    order.Status = 'O' // pending

    // Enqueue the order
    queue.SendOrder(utils.IncomingOrderQueuePath, &order)

    
    return c.JSON(http.StatusOK, map[string]interface{}{
        "status":    "Order placed successfully",
        "order_id":  order.Order_id,
        "user_id":   userID,
        "symbol":    order.Symbol,
    })
}
