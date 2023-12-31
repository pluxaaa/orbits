package ds

import (
	"L1/internal/app/role"
	"github.com/google/uuid"
	"time"
)

type User struct {
	UUID uuid.UUID `gorm:"type:uuid;unique"`
	Name string    `json:"Name"`
	Role role.Role `sql:"type:string;"`
	Pass string
}

/*
Статусы заявок ('Status'):
1. Черновик - на редактировании клиентом
2. Удалена - удалена клиентом (не отправлена, отменена)
3. На рассмотрении - отправлена клиентом, проходит проверку у модератора
4. Оказана - одобрена модератором (завершена успешно)
5. Одобрена - не одобрена модератором (завершена неуспешно)
*/

var ReqStatuses = []string{
	"Черновик",
	"На рассмотрении",
	"Удалена",
	"Отклонена",
	"Одобрена",
}

type TransferRequest struct {
	ID            uint       `gorm:"primaryKey;AUTO_INCREMENT"`
	ClientRefer   uuid.UUID  `gorm:"type:uuid"`
	Client        User       `gorm:"foreignKey:ClientRefer;references:UUID"`
	ModerRefer    uuid.UUID  `gorm:"type:uuid"`
	Moder         User       `gorm:"foreignKey:ModerRefer;references:UUID"`
	Status        string     `gorm:"type:varchar(20); not null"`
	Result        *bool      `gorm:"type:bool"`
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
	RequestRefer  uint
	Request       TransferRequest `gorm:"primaryKey;foreignKey:RequestRefer"`
	OrbitRefer    uint
	Orbit         Orbit `gorm:"primaryKey;foreignKey:OrbitRefer"`
	TransferOrder uint
}

// JSON PARSER

type ChangeTransferStatusRequestBody struct {
	TransferID uint   `json:"reqID"`
	Status     string `json:"status"`
}

type DelTransferToOrbitBody struct {
	Orbit string
	Req   string
}

type UpdateTransferOrdersBody struct {
	ReqID         int            `json:"req_id"`
	TransferOrder map[string]int `json:"transfer_order"`
}

type OrbitOrder struct {
	OrbitName     string `json:"orbit_name"`
	TransferOrder int    `json:"transfer_order"`
}

type AsyncBody struct {
	ID     int  `json:"id"`
	Status bool `json:"status"`
}
