package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jayant-dispral/brand-threat-be/services/data-service/infrastrucutre/events"
	apiHandler "github.com/jayant-dispral/brand-threat-be/services/data-service/internal/adapters/handler/http"
)

func main () {
	log.Printf("Starting data-service")

	kafkaConsumer := events.NewConsumer([]string{"kafka.paw-suite.svc.cluster.local:9092"}, "brand-moniter", "brand-workers")

	go kafkaConsumer.Listen(context.Background())

	router := apiHandler.NewRouter()

	//start the server
	srv := &http.Server{
		Addr: ":8082",
		Handler: router,
	}

	log.Printf("Server starting on port %s", "8082")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("data service failed to start server: %v", err)
	}

}
