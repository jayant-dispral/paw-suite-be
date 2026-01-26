package main

import (
	"log"
	"net/http"

	apiHandler "github.com/jayant-dispral/brand-threat-be/services/data-service/internal/adapters/handler/http"
)

func main () {
	log.Printf("Starting data-service")

	router := apiHandler.NewRouter()

	//start the server
	srv := &http.Server{
		Addr: ":8082",
		Handler: router,
	}

	log.Printf("Server starting on port %s", "8082")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("admin service failed to start server: %v", err)
	}

}
