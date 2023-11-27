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
	"slices"
	"strings"
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

	err := r.db.Where("is_available = ?", true).First(orbit, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return orbit, nil
}

func (r *Repository) GetOrbitByID(id uint) (*ds.Orbit, error) {
	orbit := &ds.Orbit{}

	err := r.db.Where("is_available = ?", true).First(orbit, "id = ?", id).Error
	if err != nil {
		return nil, err
	}

	return orbit, nil
}

func (r *Repository) GetAllOrbits(orbitName string) ([]ds.Orbit, error) {
	orbits := []ds.Orbit{}
	if orbitName == "" {
		err := r.db.Where("is_available = ?", true).
			Order("id").Find(&orbits).Error

		if err != nil {
			return nil, err
		}
	} else {
		err := r.db.Where("is_available = ?", true).Where("name ILIKE ?", "%"+orbitName+"%").
			Order("id").Find(&orbits).Error

		if err != nil {
			return nil, err
		}
	}

	return orbits, nil
}

// логическое "удаление" орбиты (SQL)
func (r *Repository) ChangeOrbitStatus(orbitName string) error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}

	tryID := "SELECT id FROM orbits WHERE name = $1"
	_, err = sqlDB.Exec(tryID, orbitName)
	if err == nil {
		query := "DELETE FROM transfer_to_orbits WHERE orbit_refer = $1"
		_, err = sqlDB.Exec(query, tryID)
		query = "UPDATE orbits SET is_available = false WHERE name = $1"
		_, err = sqlDB.Exec(query, orbitName)
		return nil
	}

	return err
}

func (r *Repository) AddOrbit(orbit *ds.Orbit, imagePath string) error {
	imageURL := "http://127.0.0.1:9000/pc-bucket/DEFAULT.jpg"

	log.Println(imagePath)
	if imagePath != "" {
		var err error
		imageURL, err = r.uploadImageToMinio(imagePath)
		if err != nil {
			return err
		}
	}

	var cntOrbits int64
	err := r.db.Model(&ds.Orbit{}).Count(&cntOrbits).Error
	if err != nil {
		return err
	}

	orbit.ImageURL = imageURL
	orbit.ID = uint(cntOrbits) + 1
	orbit.IsAvailable = true

	return r.db.Create(orbit).Error
}

func (r *Repository) EditOrbit(orbitID uint, editingOrbit ds.Orbit) error {
	// Проверяем, изменился ли URL изображения
	originalOrbit, err := r.GetOrbitByID(orbitID)
	if err != nil {
		return err
	}

	log.Println("OLD IMAGE: ", originalOrbit.ImageURL)
	log.Println("NEW IMAGE: ", editingOrbit.ImageURL)

	if editingOrbit.ImageURL != originalOrbit.ImageURL && editingOrbit.ImageURL != "" {
		log.Println("REPLACING IMAGE")
		err := r.deleteImageFromMinio(originalOrbit.ImageURL)
		if err != nil {
			return err
		}
		imageURL, err := r.uploadImageToMinio(editingOrbit.ImageURL)
		if err != nil {
			return err
		}

		editingOrbit.ImageURL = imageURL

		log.Println("IMAGE REPLACED")
	}

	return r.db.Model(&ds.Orbit{}).Where("id = ?", orbitID).Updates(editingOrbit).Error
}

func (r *Repository) uploadImageToMinio(imagePath string) (string, error) {
	minioClient := mClient.NewMinioClient()

	// Загрузка изображения в Minio
	file, err := os.Open(imagePath)
	if err != nil {
		return "!!!", err
	}
	defer file.Close()

	// uuid - уникальное имя; parts - имя файла
	//objectName := uuid.New().String() + ".jpg"
	parts := strings.Split(imagePath, "/")
	objectName := parts[len(parts)-1]

	_, err = minioClient.PutObject(context.Background(), "pc-bucket", objectName, file, -1, minio.PutObjectOptions{})
	if err != nil {
		return "!!!", err
	}

	// Возврат URL изображения в Minio
	return fmt.Sprintf("http://%s/%s/%s", minioClient.EndpointURL().Host, "pc-bucket", objectName), nil
}

func (r *Repository) deleteImageFromMinio(imageURL string) error {
	minioClient := mClient.NewMinioClient()

	objectName := extractObjectNameFromURL(imageURL)

	return minioClient.RemoveObject(context.Background(), "pc-bucket", objectName, minio.RemoveObjectOptions{})
}

func extractObjectNameFromURL(imageURL string) string {
	parts := strings.Split(imageURL, "/")
	log.Println("\n\nIMG:   ", parts[len(parts)-1])
	return parts[len(parts)-1]
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

func (r *Repository) GetRequestByID(id uint) (*ds.TransferRequest, error) {
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
func (r *Repository) GetCurrentRequest(client_refer uint) (*ds.TransferRequest, error) {
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

func (r *Repository) CreateTransferRequest(client_refer uint) (*ds.TransferRequest, error) {
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
			ModerRefer:    moder_refer,
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

func (r *Repository) ChangeRequestStatus(id uint, status string) error {
	if slices.Contains(ds.ReqStatuses[1:4], status) {
		err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("date_finished", time.Now()).Error
		if err != nil {
			return err
		}
	}

	if status == ds.ReqStatuses[4] {
		err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("date_processed", time.Now()).Error
		if err != nil {
			return err
		}
	}

	err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса: %w", err)
	}

	return nil
}

func (r *Repository) DeleteTransferRequest(req_id uint) error {
	if r.db.Where("id = ?", req_id).First(&ds.TransferRequest{}).Error != nil {

		return r.db.Where("id = ?", req_id).First(&ds.TransferRequest{}).Error
	}
	return r.db.Model(&ds.TransferRequest{}).Where("id = ?", req_id).Update("status", "Удалена").Error
}

// =================================================================================
// ---------------------------------------------------------------------------------
// ------------------------- TRANSFERS_TO_ORBITS METHODS ---------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) AddTransferToOrbits(orbit_refer, request_refer uint) error {
	orbit := ds.Orbit{ID: orbit_refer}
	request := ds.TransferRequest{ID: request_refer}

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
func (r *Repository) DeleteTransferToOrbitSingle(transfer_id uint, orbit_id uint) (error, error) {
	if r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil ||
		r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil {

		return r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error,
			r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error
	}
	return r.db.Where("request_refer = ?", transfer_id).Where("orbit_refer = ?", orbit_id).Delete(&ds.TransferToOrbit{}).Error, nil
}

// удаляет все записи по id реквеста
func (r *Repository) DeleteTransferToOrbitEvery(transfer_id uint) error {
	if r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil {
		return r.db.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error
	}
	return r.db.Where("request_refer = ?", transfer_id).Delete(&ds.TransferToOrbit{}).Error
}

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------------- USERS METHODS ---------------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) GetUserByName(name string) (*ds.User, error) {
	user := &ds.User{}

	err := r.db.First(user, "name = ?", name).Error
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *Repository) GetUserByLogin(login string) (*ds.UserUID, error) {
	user := &ds.UserUID{
		Name: "login",
	}

	err := r.db.First(user).Error
	if err != nil {
		return nil, err
	}

	return user, nil
}

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------------- AUTH METHODS ---------------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) Register(user *ds.UserUID) error {
	if user.UUID == uuid.Nil {
		user.UUID = uuid.New()
	}

	return r.db.Create(user).Error
}
