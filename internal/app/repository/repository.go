package repository

import (
	"L1/internal/app/ds"
	mClient "L1/internal/app/minio"
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"os"
	"time"
)

type Repository struct {
	db *gorm.DB
}

func New(dsn string) (*Repository, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Failed to connect to the database: %v", err)
		return nil, err
	}

	// Check the connection
	if sqlDB, err := db.DB(); err != nil {
		log.Printf("Failed to initialize the database connection: %v", err)
		return nil, err
	} else {
		if err := sqlDB.Ping(); err != nil {
			log.Printf("Failed to ping the database: %v", err)
			return nil, err
		}
	}

	return &Repository{
		db: db,
	}, nil
}

// ---------------------------------------------------------------------------------
// --------------------------------- ORBIT METHODS ---------------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) GetOrbitByName(name string) (*ds.Orbit, error) {
	orbit := &ds.Orbit{}

	err := r.db.First(orbit, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return orbit, nil
}

func (r *Repository) GetAllOrbits() ([]ds.Orbit, error) {
	orbits := []ds.Orbit{}

	err := r.db.Order("id").Find(&orbits).Error

	if err != nil {
		return nil, err
	}

	return orbits, nil
}

func (r *Repository) SearchOrbits(orbitName string) ([]ds.Orbit, error) {
	orbits := []ds.Orbit{}
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

func (r *Repository) FilterOrbits(orbits []ds.Orbit) []ds.Orbit {
	var new_orbits = []ds.Orbit{}

	for i := range orbits {
		new_orbits = append(new_orbits, orbits[i])
	}

	return new_orbits

}

func (r *Repository) AddOrbit(orbit *ds.Orbit, imagePath string) error {
	// Загрузка изображения в Minio и получение URL
	imageURL, err := r.uploadImageToMinio(imagePath)
	if err != nil {
		return err
	}

	orbit.ImageURL = imageURL

	// Добавление орбиты с путем к изображению
	return r.db.Create(orbit).Error
}

func (r *Repository) uploadImageToMinio(imagePath string) (string, error) {
	// Получаем клиента Minio из настроек
	minioClient := mClient.NewMinioClient()

	// Загрузка изображения в Minio
	file, err := os.Open(imagePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Генерация уникального имени объекта в Minio (например, используя UUID)
	objectName := uuid.New().String() + ".jpg"

	_, err = minioClient.PutObject(context.Background(), "pc-bucket", objectName, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "!!!", err
	}

	// Возврат URL изображения в Minio
	return fmt.Sprintf("http://%s/%s/%s", minioClient.EndpointURL().Host, "pc-bucket", objectName), nil
}

func (r *Repository) EditOrbit(orbitID uint, orbit ds.Orbit) error {
	log.Println("FUNC ORBIT: ", orbit, "    ", orbitID)
	return r.db.Model(&ds.Orbit{}).Where("id = ?", orbitID).Updates(orbit).Error
}

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------- TRANSFER_REQUESTS METHODS ---------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) GetAllRequests() ([]ds.TransferRequest, error) {

	requests := []ds.TransferRequest{}

	err := r.db.
		Preload("Client").Preload("Moder"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) GetRequestByID(id int) (*ds.TransferRequest, error) {
	request := &ds.TransferRequest{}

	err := r.db.First(request, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return request, nil
}

func (r *Repository) GetRequestsByStatus(status string) ([]ds.TransferRequest, error) {
	requests := []ds.TransferRequest{}

	err := r.db.
		Preload("Client").Preload("Moder"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Where("status = ?", status).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

// попытка получить заявку для конкретного клиента со статусом Черновик
func (r *Repository) GetCurrentRequest(client_refer int) (*ds.TransferRequest, error) {
	request := &ds.TransferRequest{}
	err := r.db.Where("status = ?", "Черновик").First(request, "client_refer = ?", client_refer).Error
	//если реквеста нет => err = record not found
	if err != nil {
		//request = nil, err = not found
		return nil, err
	}
	//если реквест есть => request = record, err = nil
	return request, nil
}

func (r *Repository) CreateTransferRequest(client_refer int) (*ds.TransferRequest, error) {
	//проверка есть ли открытая заявка у клиента
	request, err := r.GetCurrentRequest(client_refer)
	if err != nil {
		log.Println("NO OPENED REQUESTS => CREATING NEW ONE")

		//назначение модератора
		users := []ds.User{}
		err = r.db.Where("is_moder = ?", true).Find(&users).Error
		if err != nil {
			return nil, err
		}
		n := rand.Int() % len(users)
		moder_refer := users[n].ID
		log.Println("moder: ", moder_refer)

		//поля типа Users, связанные с передавыемыми значениями из функции
		client := ds.User{ID: uint(client_refer)}
		moder := ds.User{ID: moder_refer}

		NewTransferRequest := &ds.TransferRequest{
			ID:            uint(len([]ds.TransferRequest{})),
			ClientRefer:   client_refer,
			Client:        client,
			ModerRefer:    int(moder_refer),
			Moder:         moder,
			Status:        "Черновик",
			DateCreated:   time.Now(),
			DateProcessed: nil,
			DateFinished:  nil,
		}
		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
	}
	return request, nil
}

func (r *Repository) ChangeRequestStatus(id int, status string) error {
	return r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("status", status).Error
}

// =================================================================================
// ---------------------------------------------------------------------------------
// ------------------------- TRANSFERS_TO_ORBITS METHODS ---------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) AddTransferToOrbits(orbit_refer, request_refer int) error {
	orbit := ds.Orbit{ID: uint(orbit_refer)}
	request := ds.TransferRequest{ID: uint(request_refer)}

	NewMtM := &ds.TransferToOrbit{
		ID:           uint(len([]ds.TransferToOrbit{})),
		Orbit:        orbit,
		OrbitRefer:   orbit_refer,
		Request:      request,
		RequestRefer: request_refer,
	}
	return r.db.Create(NewMtM).Error
}

// удаляет одну запись за раз
func (r *Repository) DeleteTransferToOrbit(transfer_id int, orbit_id int) (error, error) {
	if r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil ||
		r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil {
		return r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error, r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error
	}
	return r.db.Where("request_refer = ?", transfer_id).Where("orbit_refer = ?", orbit_id).Delete(&ds.TransferToOrbit{}).Error, nil
}

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------------- USERS METHODS ---------------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) GetUserRole(name string) (*bool, error) {
	user := &ds.User{}

	err := r.db.First(user, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return user.IsModer, nil
}
