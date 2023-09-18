package ds

type Product struct {
	ID    uint `gorm:"primarykey"`
	Code  string
	Price uint
}
