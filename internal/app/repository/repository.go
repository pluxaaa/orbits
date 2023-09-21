package repository

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"L1/internal/app/ds"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &Repository{
		db: db,
	}, nil
}

func (r *Repository) GetOrbitByID(id int) (*ds.Orbits, error) {
	region := &ds.Orbits{}

	err := r.db.First(region, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return region, nil
}

func (r *Repository) SearchOrbits(orbit_name string) ([]ds.Orbits, error) {
	regions := []ds.Orbits{}

	err := r.db.Raw("select * from Orbits(?)", orbit_name).Scan(&regions).Error
	if err != nil {
		return nil, err
	}

	return regions, nil
}

func (r *Repository) GetOrbitByName(name string) (*ds.Orbits, error) {
	region := &ds.Orbits{}

	err := r.db.First(region, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return region, nil
}

func (r *Repository) GetAllOrbits() ([]ds.Orbits, error) {
	orbits := []ds.Orbits{}

	err := r.db.Find(&orbits).Error

	if err != nil {
		return nil, err
	}

	return orbits, nil
}

func (r *Repository) FilterOrbits(orbits []ds.Orbits) []ds.Orbits {
	var new_orbits = []ds.Orbits{}

	for i := range orbits {
		new_orbits = append(new_orbits, orbits[i])
	}

	return new_orbits

}

func (r *Repository) DeleteUser(user_name string) error {
	return r.db.Delete(&ds.Users{}, "name = ?", user_name).Error
}

func (r *Repository) CreateUser(user ds.Users) error {
	return r.db.Create(user).Error
}
