package repository

import (
	"L1/internal/app/ds"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"time"
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

	err := r.db.Where("name ILIKE ?", orbitName).Order("id").Find(&orbits).Error
	if err != nil {
		return nil, err
	}

	return orbits, nil
}

func (r *Repository) ChangeOrbitStatus(orbitName string) error {
	query := "UPDATE orbits SET is_available = NOT is_available WHERE Name = $1"

	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	_, err = sqlDB.Exec(query, orbitName)

	return err
}

func (r *Repository) GetAllOrbits() ([]ds.Orbits, error) {
	orbits := []ds.Orbits{}

	err := r.db.Order("id").Find(&orbits).Error

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

func (r *Repository) AddOrbit(Name, Apogee, Perigee, Inclination, Description string) error {
	NewOrbit := &ds.Orbits{
		ID:          uint(len([]ds.Orbits{})),
		Name:        Name,
		IsAvailable: true,
		Apogee:      Apogee,
		Perigee:     Perigee,
		Inclination: Inclination,
		Description: Description,
		Image:       "",
	}

	return r.db.Create(NewOrbit).Error
}

func (r *Repository) EditOrbitName(oldName, newName string) error {
	return r.db.Model(&ds.Orbits{}).Where(
		"name", oldName).Update(
		"name", newName).Error
}

func (r *Repository) GetCurrentRequest(client_refer int) (*ds.TransferRequests, error) {
	request := &ds.TransferRequests{}
	err := r.db.Where("status = ?", "Opened").First(request, "client_refer = ?", client_refer).Error
	//если реквеста нет => err = record not found
	if err != nil {
		//request = nil, err = not found
		return nil, err
	}
	//если реквест есть => request = record, err = nil
	return request, nil
}

func (r *Repository) CreateTransferRequest(client_refer int) (*ds.TransferRequests, error) {
	//проверка есть ли открытая заявка у клиента
	request, err := r.GetCurrentRequest(client_refer)
	if err != nil {
		log.Println("NO OPENED REQUESTS => CREATING NEW ONE")

		//назначение модератора
		users := []ds.Users{}
		err = r.db.Where("is_moder = ?", true).Find(&users).Error
		if err != nil {
			return nil, err
		}
		n := rand.Int() % len(users)
		moder_refer := users[n].ID

		//поля типа Users, связанные с передавыемыми значениями из функции
		client := ds.Users{ID: uint(client_refer)}
		moder := ds.Users{ID: moder_refer}

		NewTransferRequest := &ds.TransferRequests{
			ID:            uint(len([]ds.TransferRequests{})),
			ClientRefer:   client_refer,
			Client:        client,
			ModerRefer:    int(moder_refer),
			Moder:         moder,
			Status:        "Opened",
			DateCreated:   time.Now(),
			DateProcessed: nil,
			DateFinished:  nil,
		}
		log.Println("!!! NEW RECORD ADDED")
		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
	}
	return request, nil
}

func (r *Repository) AddTransferToOrbits(orbit_refer, request_refer int) error {
	orbit := ds.Orbits{ID: uint(orbit_refer)}
	request := ds.TransferRequests{ID: uint(request_refer)}

	NewMtM := &ds.TransfersToOrbit{
		ID:           uint(len([]ds.TransfersToOrbit{})),
		Orbit:        orbit,
		OrbitRefer:   orbit_refer,
		Request:      request,
		RequestRefer: request_refer,
	}

	return r.db.Create(NewMtM).Error
}
