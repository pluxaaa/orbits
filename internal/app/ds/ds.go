package ds

import (
	"time"
)

type User struct {
	ID       uint `gorm:"primaryKey;AUTO_INCREMENT"`
	IsModer  *bool
	Name     string `gorm:"type:varchar(50);unique;not null"`
	Password string `gorm:"type:varchar(50);not null"`
}

/*
Статусы заявок ('Status'):
1. Черновик - на редактировании клиентом
2. Удалена - удалена клиентом (не отправлена, отменена)
3. На рассмотрении - отправлена клиентом, проходит проверку у модератора
4. Оказана - одобрена модератором (завершена успешно)
5. Отклонена - не одобрена модератором (завершена неуспешно)
*/
type TransferRequest struct {
	ID            uint `gorm:"primaryKey;AUTO_INCREMENT"`
	ClientRefer   int
	Client        User `gorm:"foreignKey:ClientRefer"`
	ModerRefer    int
	Moder         User       `gorm:"foreignKey:ModerRefer"`
	Status        string     `gorm:"type:varchar(20); not null"`
	DateCreated   time.Time  `gorm:"type:timestamp"` //timestamp without time zone
	DateProcessed *time.Time `gorm:"type:timestamp"`
	DateFinished  *time.Time `gorm:"type:timestamp"`
}

type Orbit struct {
	ID          uint   `gorm:"primaryKey;AUTO_INCREMENT"`
	Name        string `gorm:"type:varchar(50)"`
	IsAvailable bool
	Apogee      string `gorm:"type:varchar(20)"`
	Perigee     string `gorm:"type:varchar(20)"`
	Inclination string `gorm:"type:varchar(20)"`
	Description string `gorm:"type:text"`
	Image       string `gorm:"type:bytea"`
}

type TransferToOrbit struct {
	ID           uint `gorm:"primaryKey;AUTO_INCREMENT"`
	RequestRefer int
	Request      TransferRequest `gorm:"foreignKey:RequestRefer"`
	OrbitRefer   int
	Orbit        Orbit `gorm:"foreignKey:OrbitRefer"`
}

// JSON PARSER

type ChangeTransferStatusRequestBody struct {
	TransferID int
	Status     string
	UserName   string
}
