package handler

import (
	"fmt"
	"os"
	"time"

	"equipment-telemetry-simulator/internal/model"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DbConnect() (*gorm.DB, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Println("WARNING: .env not found, system environment variables will be used")
	}

	host := envOrDefault("DB_HOST", "localhost")
	port := envOrDefault("DB_PORT", "5432")
	user := envOrDefault("DB_USER", "postgres")
	password := os.Getenv("DB_PASSWORD")
	dbName := envOrDefault("DB_NAME", "equipment_telemetry")
	sslMode := envOrDefault("DB_SSL", "disable")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		host,
		port,
		user,
		password,
		dbName,
		sslMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("read sql database handle: %w", err)
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err := db.AutoMigrate(&model.AssetTypeDefinition{}, &model.Asset{}); err != nil {
		return nil, fmt.Errorf("auto migrate database schema: %w", err)
	}

	return db, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
