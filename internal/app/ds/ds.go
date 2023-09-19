package ds

import "gorm.io/datatypes"

type Product struct {
	ID    uint `gorm:"primaryKey"`
	Code  string
	Price uint
}

type Test struct {
	ID           uint `gorm:"primaryKey"`
	ProductRefer int
	Product      Product `gorm:"foreignKey:ProductRefer"`
	TestField    string
}

type DateTest struct {
	ID            uint `gorm:"primaryKey"`
	DateTestField datatypes.Date
}
