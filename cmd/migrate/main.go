package main

import (
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"L1/internal/app/ds"
	"L1/internal/app/dsn"
)

func main() {
	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(dsn.FromEnv()), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema

	err = db.AutoMigrate(&ds.Users{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.TransferRequest{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.Orbits{})
	if err != nil {
		panic("cant migrate db")
	}

	err = db.AutoMigrate(&ds.TransferToOrbit{})
	if err != nil {
		panic("cant migrate db")
	}
}
