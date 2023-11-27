package ds

import (
	"L1/internal/app/role"
	"github.com/google/uuid"
	"time"
)

type User struct {
	UUID uuid.UUID `gorm:"type:uuid;unique"`
	Name string    `json:"name"`
	Role role.Role `sql:"type:string;"`
	Pass string
}

/*
Статусы заявок ('Status'):
1. Черновик - на редактировании клиентом
2. Удалена - удалена клиентом (не отправлена, отменена)
3. На рассмотрении - отправлена клиентом, проходит проверку у модератора
4. Оказана - одобрена модератором (завершена успешно)
5. Отклонена - не одобрена модератором (завершена неуспешно)
*/

var ReqStatuses = []string{"Черновик", "Удалена", "Отклонена", "Оказана", "На рассмотрении"}

type TransferRequest struct {
	ID            uint       `gorm:"primaryKey;AUTO_INCREMENT"`
	ClientRefer   uuid.UUID  `gorm:"type:uuid"`
	Client        User       `gorm:"foreignKey:ClientRefer;references:UUID"`
	ModerRefer    uuid.UUID  `gorm:"type:uuid"`
	Moder         User       `gorm:"foreignKey:ModerRefer;references:UUID"`
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
	ImageURL    string `gorm:"column:image"`
}

type TransferToOrbit struct {
	ID           uint `gorm:"primaryKey;AUTO_INCREMENT"`
	RequestRefer uint
	Request      TransferRequest `gorm:"foreignKey:RequestRefer"`
	OrbitRefer   uint
	Orbit        Orbit `gorm:"foreignKey:OrbitRefer"`
}

// JSON PARSER

type ChangeTransferStatusRequestBody struct {
	TransferID uint
	Status     string
	UserName   string
}
