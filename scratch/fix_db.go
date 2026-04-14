package main

import (
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
	
	result := storage.DB.Model(&models.Job{}).Where("1 = 1").Update("strict_mode", true)
	if result.Error != nil {
		log.Fatalf("Update failed: %v", result.Error)
	}
	
	log.Printf("Updated %d jobs to StrictMode = true", result.RowsAffected)
}
