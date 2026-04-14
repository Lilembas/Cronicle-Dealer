package main

import (
	"fmt"
	"log"

	"github.com/cronicle/cronicle-next/internal/config"
	"github.com/cronicle/cronicle-next/internal/models"
	"github.com/cronicle/cronicle-next/internal/storage"
)

func main() {
	cfg := &config.DatabaseConfig{
		Driver: "sqlite",
		Path:   "cronicle.db",
	}
	
	if err := storage.InitDB(cfg); err != nil {
		log.Fatalf("InitDB failed: %v", err)
	}
	
	var jobs []models.Job
	if err := storage.DB.Find(&jobs).Error; err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	
	for _, job := range jobs {
		fmt.Printf("Job ID: %s, Name: %s, Command: %s, StrictMode: %v\n", job.ID, job.Name, job.Command, job.StrictMode)
	}
}
