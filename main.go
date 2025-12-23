package main

import (
	"database/sql"
	"jotacomputing/go-api/db"
	"jotacomputing/go-api/handlers"
	"jotacomputing/go-api/queue"
	"net/http"
	"strconv"

	echoserver "github.com/dasjott/oauth2-echo-server"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/labstack/echo/v4"
)



func main() {
	queue.InitQueues()
	defer queue.CloseQueues()

    // Initialize the database
    db.InitDB()
    // OAuth2 Server Setup
    manager := manage.NewDefaultManager()
    manager.MustTokenStorage(store.NewFileTokenStore("data.db")) // Persists tokens

    // Register OAuth2 client (your API gateway app)
    clientStore := store.NewClientStore()
    clientStore.Set("stock-app", &models.Client{
        ID:     "stock-app",
        Secret: "supersecret", // Change in production (use env vars)
        Domain: "http://localhost:1323",
    })
    manager.MapClientStorage(clientStore)

    // Initialize Echo OAuth2 server
    echoserver.InitServer(manager)
    echoserver.SetAllowGetAccessRequest(true)
    echoserver.SetClientInfoHandler(server.ClientFormHandler)

    // User authorization handler (maps username to userID)
    echoserver.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
    username := r.FormValue("username")
    
    user, err := db.FindUserByUsername(username)
    if err == sql.ErrNoRows {
        // NEW USER → auto-generates ID
        user, err = db.CreateUser(username, 0.0)
    }
    
    if err != nil {
        http.Error(w, "user error", http.StatusInternalServerError)
        return "", err
    }
    
    return strconv.FormatUint(user.ID, 10), nil // "1001"
})




    e := echo.New()

    // OAuth2 endpoints - clients hit these to get tokens
    oauth := e.Group("/oauth2")
    oauth.POST("/token", echoserver.HandleTokenRequest) // Main token endpoint



    // Protected API routes
    api := e.Group("/api")
    api.Use(echoserver.TokenHandler()) // Validates Bearer token on every request
    
    api.POST("/order", handlers.PostOrderHandler)
    api.GET("/balance/:userID", handlers.GetBalanceHandler)
    api.GET("/holdings/:userID", handlers.GetHoldingsHandler)
    api.DELETE("/cancel/:orderId", handlers.CancelOrderHandler)

    e.Logger.Fatal(e.Start(":1323"))
}
