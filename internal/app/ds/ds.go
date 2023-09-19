package ds

import "gorm.io/datatypes"

type Roles struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

type Users struct {
	ID        uint `gorm:"primaryKey"`
	RoleRefer int
	Role      Roles `gorm:"foreignKey:RoleRefer"`
	Name      string
}

type TransferRequest struct {
	ID             uint `gorm:"primaryKey"`
	ClientRefer    int
	Client         Users `gorm:"foreignKey:ClientRefer"`
	ModerRefer     int
	Moder          Users `gorm:"foreignKey:ModerRefer"`
	Status         string
	MissionPurpose string
	DateCreated    datatypes.Date
	DateProcessed  datatypes.Date
	DateFinished   datatypes.Date
}

type Orbits struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Image string `gorm:"type:bytea"`
}

type TransferToOrbit struct {
	ID           uint `gorm:"primaryKey"`
	RequestRefer int
	Request      TransferRequest `gorm:"foreignKey:RequestRefer"`
	OrbitRefer   int
	Orbit        Orbits `gorm:"foreignKey:OrbitRefer"`
	Name         string
}
