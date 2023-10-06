package ds

import (
	"time"
)

type Users struct {
	ID      uint   `gorm:"primaryKey;AUTO_INCREMENT"`
	IsModer bool   `gorm:"not null"`
	Name    string `gorm:"type:varchar(50);unique;not null"`
}

type TransferRequests struct {
	ID             uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ClientRefer    int
	Client         Users `gorm:"foreignKey:ClientRefer"`
	ModerRefer     int
	Moder          Users     `gorm:"foreignKey:ModerRefer"`
	Status         string    `gorm:"type:varchar(20);not null"`
	MissionPurpose string    `gorm:"type:varchar(50)"`
	DateCreated    time.Time `gorm:"type:timestamp"` //timestamp without time zone
	DateProcessed  time.Time `gorm:"type:timestamp"`
	DateFinished   time.Time `gorm:"type:timestamp"`
}

type Orbits struct {
	ID          uint   `gorm:"primaryKey;AUTO_INCREMENT"`
	Name        string `gorm:"type:varchar(50)"`
	IsAvailable bool
	Apogee      string `gorm:"type:varchar(20)"`
	Perigee     string `gorm:"type:varchar(20)"`
	Inclination string `gorm:"type:varchar(20)"`
	Description string `gorm:"type:text"`
	Image       string `gorm:"type:bytea"`
}

type TransfersToOrbit struct {
	ID           uint `gorm:"primaryKey;AUTO_INCREMENT"`
	RequestRefer int
	Request      TransferRequests `gorm:"foreignKey:RequestRefer"`
	OrbitRefer   int
	Orbit        Orbits `gorm:"foreignKey:OrbitRefer"`
}

// JSON PARSER
type AddOrbitRequestBody struct {
	Name        string
	Apogee      string
	Perigee     string
	Inclination string
	Description string
}

type EditOrbitNameRequestBody struct {
	OldName string
	NewName string
}
