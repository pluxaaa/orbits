package repository

import (
	"L1/internal/app/ds"
	mClient "L1/internal/app/minio"
	"L1/internal/app/role"
	"context"
	"crypto/sha1"
	"encoding/hex"
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

func (r *Repository) GetAllOrbits(orbitName, orbitIncl, isCircle string) ([]ds.Orbit, error) {
	orbits := []ds.Orbit{}
	qry := r.db
	if orbitName != "" {
		log.Println("orbitName")
		qry = qry.Where("name ILIKE ?", "%"+orbitName+"%")
	}

	if orbitIncl != "" {
		log.Println("incl")
		qry = qry.Where("inclination::float > ?", orbitIncl)
	}

	if isCircle != "" {
		log.Println("circle")
		if isCircle == "1" {
			qry = qry.Where("apogee = perigee")
		} else {
			qry = qry.Where("apogee != perigee")
		}
	}
	qry = qry.Where("is_available = ?", true)

	err := qry.Order("name").Find(&orbits).Error

	if err != nil {
		return nil, err
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

		if originalOrbit.ImageURL != "http://127.0.0.1:9000/pc-bucket/DEFAULT.jpg" {
			err := r.deleteImageFromMinio(originalOrbit.ImageURL)
			if err != nil {
				return err
			}
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

func (r *Repository) GetAllRequests(userRole any, dateStart, dateFin, status string) ([]ds.TransferRequest, error) {

	requests := []ds.TransferRequest{}
	qry := r.db

	if dateStart != "" && dateFin != "" {
		qry = qry.Where("date_processed BETWEEN ? AND ?", dateStart, dateFin)
	} else if dateStart != "" {
		qry = qry.Where("date_processed >= ?", dateStart)
	} else if dateFin != "" {
		qry = qry.Where("date_processed <= ?", dateFin)
	}

	if status != "" {
		qry = qry.Where("status = ?", status)
	}

	if userRole == role.Moderator {
		qry = qry.Where("status = ?", ds.ReqStatuses[1])
	} else {
		qry = qry.Where("status IN ?", ds.ReqStatuses)
	}

	err := qry.
		Preload("Client").Preload("Moder"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}
	log.Println(requests)

	return requests, nil
}

func (r *Repository) GetRequestByID(id uint, userUUID uuid.UUID, userRole any) (*ds.TransferRequest, error) {
	request := &ds.TransferRequest{}
	qry := r.db

	if userRole == role.Client {
		qry = qry.Where("client_refer = ?", userUUID)
	} else {
		qry = qry.Where("moder_refer = ?", userUUID)
	}

	err := qry.Preload("Client").Preload("Moder").First(request, "id = ?", id).Error
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
func (r *Repository) GetCurrentRequest(client_refer uuid.UUID) (*ds.TransferRequest, error) {
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

// старое создание заявки -> удалить?
//func (r *Repository) CreateTransferRequest(client_id uuid.UUID) (*ds.TransferRequest, error) {
//	//проверка есть ли открытая заявка у клиента
//	request, err := r.GetCurrentRequest(client_id)
//	if err != nil {
//		log.Println("NO OPENED REQUESTS => CREATING NEW ONE")
//
//		//назначение модератора
//		moders := []ds.User{}
//		err = r.db.Where("role = ?", 2).Find(&moders).Error
//		if err != nil {
//			return nil, err
//		}
//		n := rand.Int() % len(moders)
//		moder_refer := moders[n].UUID
//		log.Println("moder: ", moder_refer)
//
//		//поля типа Users, связанные с передавыемыми значениями из функции
//		client := ds.User{UUID: client_id}
//		moder := ds.User{UUID: moder_refer}
//
//		NewTransferRequest := &ds.TransferRequest{
//			ID:            uint(len([]ds.TransferRequest{})),
//			ClientRefer:   client_id,
//			Client:        client,
//			ModerRefer:    moder_refer,
//			Moder:         moder,
//			Status:        "Черновик",
//			DateCreated:   time.Now(),
//			DateProcessed: nil,
//			DateFinished:  nil,
//		}
//		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
//	}
//	return request, nil
//}

// новое создание заявки
func (r *Repository) CreateTransferRequest(requestBody ds.CreateTransferRequestBody, userUUID uuid.UUID) (int, error) {
	var orbit_ids []int
	var orbit_names []string
	for _, orbitName := range requestBody.Orbits {
		orbit, err := r.GetOrbitByName(orbitName)
		if err != nil {
			return 0, err
		}
		orbit_ids = append(orbit_ids, int(orbit.ID))
		orbit_names = append(orbit_names, orbit.Name)
	}

	request, err := r.GetCurrentRequest(userUUID)
	if err != nil {
		log.Println(" --- NEW REQUEST --- ", userUUID)

		// Назначение модератора
		moders := []ds.User{}
		err = r.db.Where("role = ?", 2).Find(&moders).Error
		if err != nil {
			return 0, err
		}
		n := rand.Int() % len(moders)
		moder_refer := moders[n].UUID
		log.Println("moder: ", moder_refer)

		// Поля типа Users, связанные с передаваемыми значениями из функции
		client := ds.User{UUID: userUUID}
		moder := ds.User{UUID: moder_refer}

		request = &ds.TransferRequest{
			ID:            uint(len([]ds.TransferRequest{})),
			ClientRefer:   userUUID,
			Client:        client,
			ModerRefer:    moder_refer,
			Moder:         moder,
			Status:        "Черновик",
			DateCreated:   time.Now(),
			DateProcessed: nil,
			DateFinished:  nil,
		}

		err := r.db.Create(request).Error
		if err != nil {
			return 0, err
		}
	}

	err = r.SetRequestOrbits(int(request.ID), orbit_names)
	if err != nil {
		return 0, err
	}

	// Uncomment the following block if needed
	// for _, orbit_id := range orbit_ids {
	// 	transfer_to_orbit := ds.TransferToOrbit{}
	// 	transfer_to_orbit.RequestRefer = request.ID
	// 	transfer_to_orbit.OrbitRefer = uint(orbit_id)
	// 	err = r.CreateTransferToOrbit(transfer_to_orbit)
	//
	// 	if err != nil {
	// 		return 0, err
	// 	}
	// }

	// Return request ID along with nil error
	return int(request.ID), nil
}

func (r *Repository) SetRequestOrbits(transferID int, orbits []string) error {
	var orbit_ids []int
	log.Println(transferID, " - ", orbits)
	for _, orbit_name := range orbits {
		orbit, err := r.GetOrbitByName(orbit_name)
		log.Println("orbit: ", orbit)
		if err != nil {
			return err
		}

		for _, ele := range orbit_ids {
			if ele == int(orbit.ID) {
				log.Println("!!!")
				continue
			}
		}
		log.Println("BEFORE :", orbit_ids)
		orbit_ids = append(orbit_ids, int(orbit.ID))
		log.Println("AFTER :", orbit_ids)
	}

	var existing_links []ds.TransferToOrbit
	err := r.db.Model(&ds.TransferToOrbit{}).Where("request_refer = ?", transferID).Find(&existing_links).Error
	if err != nil {
		return err
	}
	log.Println("LINKS: ", existing_links)
	for _, link := range existing_links {
		orbitFound := false
		orbitIndex := -1
		for index, ele := range orbit_ids {
			if ele == int(link.OrbitRefer) {
				orbitFound = true
				orbitIndex = index
				break
			}
		}
		log.Println("ORB F: ", orbitFound)
		if orbitFound {
			log.Println("APPEND: ")
			orbit_ids = append(orbit_ids[:orbitIndex], orbit_ids[orbitIndex+1:]...)
		} else {
			log.Println("DELETE: ")
			err := r.db.Model(&ds.TransferToOrbit{}).Delete(&link).Error
			if err != nil {
				return err
			}
		}
	}

	for _, orbit_id := range orbit_ids {
		newLink := ds.TransferToOrbit{
			RequestRefer: uint(transferID),
			OrbitRefer:   uint(orbit_id),
		}
		log.Println("NEW LINK", newLink.OrbitRefer, " --- ", newLink.RequestRefer)
		err := r.db.Model(&ds.TransferToOrbit{}).Create(&newLink).Error
		if err != nil {
			return nil
		}
	}

	return nil
}

func (r *Repository) ChangeRequestStatus(id uint, status string) error {
	if slices.Contains(ds.ReqStatuses[2:5], status) {
		err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("date_finished", time.Now()).Error
		if err != nil {
			return err
		}
	}

	if status == ds.ReqStatuses[1] {
		err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("date_processed", time.Now()).Error
		if err != nil {
			return err
		}
	}

	err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return fmt.Errorf("ошибка обновления статуса: %w", err)
	}

	if status == ds.ReqStatuses[2] || status == ds.ReqStatuses[3] {
		err = r.DeleteTransferToOrbitEvery(id)
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

// новое создание м-м
func (r *Repository) CreateTransferToOrbit(transfer_to_orbit ds.TransferToOrbit) error {
	return r.db.Create(&transfer_to_orbit).Error
}

func (r *Repository) GetOrbitsFromTransfer(id int) ([]ds.Orbit, error) {
	transfer_to_orbits := []ds.TransferToOrbit{}

	err := r.db.Model(&ds.TransferToOrbit{}).Where("request_refer = ?", id).Find(&transfer_to_orbits).Error
	if err != nil {
		return []ds.Orbit{}, err
	}

	var orbits []ds.Orbit
	for _, transfer_to_orbit := range transfer_to_orbits {
		orbit, err := r.GetOrbitByID(transfer_to_orbit.OrbitRefer)
		if err != nil {
			return []ds.Orbit{}, err
		}
		for _, ele := range orbits {
			if ele == *orbit {
				continue
			}
		}
		orbits = append(orbits, *orbit)
	}

	return orbits, nil

}

// старое создание м-м -> не исп скорее всего -> удалить?
//func (r *Repository) AddTransferToOrbits(orbit_refer, request_refer uint) error {
//	orbit := ds.Orbit{ID: orbit_refer}
//	request := ds.TransferRequest{ID: request_refer}
//
//	NewMtM := &ds.TransferToOrbit{
//		ID:           uint(len([]ds.TransferToOrbit{})),
//		Orbit:        orbit,
//		OrbitRefer:   orbit_refer,
//		Request:      request,
//		RequestRefer: request_refer,
//	}
//	return r.db.Create(NewMtM).Error
//}

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

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------------- AUTH METHODS ---------------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) Register(user *ds.User) error {
	if user.UUID == uuid.Nil {
		user.UUID = uuid.New()
	}

	return r.db.Create(user).Error
}

func (r *Repository) GenerateHashString(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
