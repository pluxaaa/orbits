package repository

import (
	"L1/internal/app/ds"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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
	Orbit := &ds.Orbits{}

	err := r.db.First(Orbit, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return Orbit, nil
}

func (r *Repository) SearchOrbits(orbitName string) ([]ds.Orbits, error) {
	orbits := []ds.Orbits{}
	orbitName = "%" + orbitName + "%"

	err := r.db.Where("name ILIKE ?", orbitName).Find(&orbits).Error
	if err != nil {
		return nil, err
	}

	return orbits, nil
}

func (r *Repository) DeleteOrbit(orbit_name string) error {
	return r.db.Delete(&ds.Orbits{}, "name = ?", orbit_name).Error
}

func (r *Repository) ChangeAvailability(orbitName string) error {
	// Получите текущее значение поля Free для указанной орбиты
	orbit := ds.Orbits{}
	if err := r.db.Where("name = ?", orbitName).First(&orbit).Error; err != nil {
		return err
	}

	// Инвертируйте значение поля Free
	orbit.Free = !orbit.Free

	// Обновите значение поля Free в базе данных
	if err := r.db.Save(&orbit).Error; err != nil {
		return err
	}

	return nil
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

func (r *Repository) GetOrbitByName(name string) (*ds.Orbits, error) {
	orbit := &ds.Orbits{}

	err := r.db.First(orbit, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return orbit, nil
}

func (r *Repository) DeleteUser(user_name string) error {
	return r.db.Delete(&ds.Users{}, "name = ?", user_name).Error
}

func (r *Repository) CreateUser(user ds.Users) error {
	return r.db.Create(user).Error
}
