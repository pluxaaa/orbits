package main

import (
	"L1/internal/app/ds"
	"L1/internal/app/dsn"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema

	err = db.AutoMigrate(&ds.User{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.TransferRequest{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.Orbit{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.TransferToOrbit{})
	if err != nil {
		panic("cant migrate db")
	}

}
