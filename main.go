package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"jotacomputing/go-api/db"
	"jotacomputing/go-api/handlers"
	"jotacomputing/go-api/queue"
	"jotacomputing/go-api/utils"

	echoserver "github.com/dasjott/oauth2-echo-server"
	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-oauth2/oauth2/v4/store"
	"github.com/labstack/echo/v4"
)

func main() {
	// Initialize queues
	queue.InitQueue(utils.IncomingOrderQueuePath)
	queue.InitCancelQueue(utils.CancelOrderQueuePath)
	queue.InitQueryQueue(utils.QueryQueuePath)

	if err := queue.InitQueues(); err != nil {
		log.Fatalf("Failed to initialize queues: %v", err)
	}
	defer queue.CloseQueues()

	db.InitDB()

	// OAuth2 Server Setup
	manager := manage.NewDefaultManager()
	manager.MustTokenStorage(store.NewFileTokenStore("data.db"))

	// Register OAuth2 client
	clientStore := store.NewClientStore()
	clientStore.Set("stock-app", &models.Client{
		ID:     "stock-app",
		Secret: "supersecret",
		Domain: "http://localhost:1323",
	})
	manager.MapClientStorage(clientStore)

	// Init Echo OAuth2 server
	echoserver.InitServer(manager)
	echoserver.SetAllowGetAccessRequest(true)
	echoserver.SetClientInfoHandler(server.ClientFormHandler)

	// 1) Allow PASSWORD grant
	echoserver.SetAllowedGrantType(oauth2.PasswordCredentials)

	// 2) Optional: ensure this client may use password grant
	echoserver.SetClientAuthorizedHandler(func(clientID string, grant oauth2.GrantType) (bool, error) {
		if clientID == "stock-app" && grant == oauth2.PasswordCredentials {
			return true, nil
		}
		return false, nil
	})

	// 3) PasswordAuthorizationHandler: check username/password, return user ID
	echoserver.SetPasswordAuthorizationHandler(
		func(ctx context.Context, clientID, username, password string) (string, error) {
			// For testing: accept any non-empty username/password
			if username == "" || password == "" {
				return "", echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
			}

			// Optionally verify clientID
			if clientID != "stock-app" {
				return "", echo.NewHTTPError(http.StatusUnauthorized, "invalid client")
			}

			// Find or create user in database
			user, err := db.FindUserByUsername(username)
			if err == sql.ErrNoRows {
				// New user → create with default balance 0.0
				user, err = db.CreateUser(username, 0.0)
			}
			if err != nil {
				return "", err
			}

			// TODO: Add real password verification here
			// if !verifyPassword(user.PasswordHash, password) {
			//     return "", echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
			// }

			// Return user ID as string - this becomes ti.GetUserID() in handlers
			return strconv.FormatUint(user.ID, 10), nil
		},
	)
	/*  // 4) UserAuthorizationHandler: map that username → internal uint64 user ID (DB row)
	    echoserver.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (string, error) {
	        // The password handler above returned `username` as the "user id".
	        // That same username is now available in the request.
	        username := r.FormValue("username")

	        user, err := db.FindUserByUsername(username)
	        if err == sql.ErrNoRows {
	            // New user → create with default balance 0.0
	            user, err = db.CreateUser(username, 0.0)
	        }
	        if err != nil {
	            http.Error(w, "user error", http.StatusInternalServerError)
	            return "", err
	        }

	        // This string becomes ti.GetUserID() in handlers
	        return strconv.FormatUint(user.ID, 10), nil
	    }) */

	e := echo.New()

	// OAuth2 endpoint
	oauth := e.Group("/oauth2")
	oauth.POST("/token", echoserver.HandleTokenRequest)

	// Protected routes
	api := e.Group("/api")
	api.Use(echoserver.TokenHandler())

	api.POST("/order", handlers.PostOrderHandler)
	api.GET("/balance/:userID", handlers.GetBalanceHandler)
	api.GET("/holdings/:userID", handlers.GetHoldingsHandler)
	api.DELETE("/cancel/:orderId", handlers.CancelOrderHandler)

	e.Logger.Fatal(e.Start(":1323"))
}
