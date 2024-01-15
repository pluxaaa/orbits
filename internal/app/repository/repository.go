package repository

import (
	"L1/internal/app/ds"
	mClient "L1/internal/app/minio"
	"L1/internal/app/role"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"math/rand"
	"net/http"
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

func (r *Repository) GetAllOrbits(orbitName, orbitIncl, isCircle string, userUUID uuid.UUID) ([]ds.Orbit, uint, error) {
	orbits := []ds.Orbit{}
	var reqID uint
	qry := r.db
	if orbitName != "" {
		qry = qry.Where("name ILIKE ?", "%"+orbitName+"%")
	}

	if orbitIncl != "0" && orbitIncl != "" {
		qry = qry.Where("inclination::float > ?", 0)
	}

	if isCircle != "" {
		if isCircle == "1" {
			qry = qry.Where("apogee = perigee")
		} else {
			qry = qry.Where("apogee != perigee")
		}
	}
	qry = qry.Where("is_available = ?", true)

	err := qry.Order("name").Find(&orbits).Error

	if err != nil {
		return nil, 0, err
	}

	request, err := r.GetCurrentRequest(userUUID)
	if err != nil {
		reqID = 0
	} else {
		reqID = request.ID
	}

	return orbits, reqID, nil
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

func (r *Repository) AddOrbit(orbit *ds.Orbit, imageURL string) error {
	if imageURL == "" {
		imageURL = "http://127.0.0.1:9000/pc-bucket/DEFAULT.jpg"
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

	if editingOrbit.ImageURL != originalOrbit.ImageURL && editingOrbit.ImageURL != "" {

		if originalOrbit.ImageURL != "http://127.0.0.1:9000/pc-bucket/DEFAULT.jpg" {
			err := r.deleteImageFromMinio(originalOrbit.ImageURL)
			if err != nil {
				return err
			}
		}
	}

	return r.db.Model(&ds.Orbit{}).Where("id = ?", orbitID).Updates(editingOrbit).Error
}

func (r *Repository) UploadImageToMinio(imagePath, orbitName string) (string, error) {
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
	imgURL := fmt.Sprintf("http://%s/%s/%s", minioClient.EndpointURL().Host, "pc-bucket", objectName)
	err = r.db.Model(&ds.Orbit{}).Where("name = ?", orbitName).Update("image", imgURL).Error

	return imgURL, nil
}

func (r *Repository) deleteImageFromMinio(imageURL string) error {
	minioClient := mClient.NewMinioClient()

	objectName := extractObjectNameFromURL(imageURL)

	return minioClient.RemoveObject(context.Background(), "pc-bucket", objectName, minio.RemoveObjectOptions{})
}

func extractObjectNameFromURL(imageURL string) string {
	parts := strings.Split(imageURL, "/")
	return parts[len(parts)-1]
}

// =================================================================================
// ---------------------------------------------------------------------------------
// --------------------------- TRANSFER_REQUESTS METHODS ---------------------------
// ---------------------------------------------------------------------------------

func (r *Repository) GetAllRequests(userRole any, dateStart, dateFin, status /*client*/ string) ([]ds.TransferRequest, error) {

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
		if status == "client" {
			qry = qry.Where("status NOT IN (?, ?)", "Черновик", "Удалена")
		} else {
			qry = qry.Where("status = ?", status)
		}
	}

	//if client != "" {
	//	clientUUID, err := r.GetUserByName(client)
	//	if err != nil {
	//		return nil, err
	//	}
	//	qry = qry.Where("client_refer = ?", clientUUID.UUID)
	//}

	if userRole == role.Moderator {
		qry = qry.Where("status = ?", ds.ReqStatuses[1])
	}

	err := qry.
		Preload("Client").Preload("Moder"). //данные для полей типа User: {ID, Name, IsModer)
		Order("id").
		Find(&requests).Error

	if err != nil {
		return nil, err
	}

	return requests, nil
}

func (r *Repository) CreateTransferRequest(client_id uuid.UUID) (*ds.TransferRequest, error) {
	//проверка есть ли открытая заявка у клиента
	request, err := r.GetCurrentRequest(client_id)
	if err != nil {

		//назначение модератора
		moders := []ds.User{}
		err = r.db.Where("role = ?", 2).Find(&moders).Error
		if err != nil {
			return nil, err
		}
		n := rand.Int() % len(moders)
		moder_refer := moders[n].UUID

		//поля типа Users, связанные с передавыемыми значениями из функции
		client := ds.User{UUID: client_id}
		moder := ds.User{UUID: moder_refer}

		NewTransferRequest := &ds.TransferRequest{
			ID:            uint(len([]ds.TransferRequest{})),
			ClientRefer:   client_id,
			Client:        client,
			ModerRefer:    moder_refer,
			Moder:         moder,
			Status:        "Черновик",
			DateCreated:   time.Now(),
			DateProcessed: nil,
			DateFinished:  nil,
			Result:        nil,
		}
		return NewTransferRequest, r.db.Create(NewTransferRequest).Error
	}
	return request, nil
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

//func (r *Repository) CreateTransferRequest(client_id uuid.UUID) (uint, error) {
//	//назначение модератора
//	moders := []ds.User{}
//	err := r.db.Where("role = ?", 2).Find(&moders).Error
//	if err != nil {
//		return 0, err
//	}
//	n := rand.Int() % len(moders)
//	moder_refer := moders[n].UUID
//
//	//поля типа Users, связанные с передавыемыми значениями из функции
//	client := ds.User{UUID: client_id}
//	moder := ds.User{UUID: moder_refer}
//
//	NewTransferRequest := &ds.TransferRequest{
//		ID:            uint(len([]ds.TransferRequest{})),
//		ClientRefer:   client_id,
//		Client:        client,
//		ModerRefer:    moder_refer,
//		Moder:         moder,
//		Status:        "Черновик",
//		DateCreated:   time.Now(),
//		DateProcessed: nil,
//		DateFinished:  nil,
//		Result:        nil,
//	}
//	return NewTransferRequest.ID, r.db.Create(NewTransferRequest).Error
//}

func (r *Repository) ChangeRequestStatus(id uint, status string) error {
	if slices.Contains(ds.ReqStatuses[2:5], status) {
		err := r.db.Model(&ds.TransferRequest{}).Where("id = ?", id).Update("date_finished", time.Now()).Error
		if err != nil {
			return err
		}
	}

	// расчет успеха маневра на выделенном сервисе
	if status == "На рассмотрении" {
		err := r.GetTransferRequestResult(id)
		if err != nil {
			fmt.Println("Ошибка при отправлении запроса:", err)
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

	if status == ds.ReqStatuses[2] {
		err = r.DeleteTransferToOrbitEvery(id)
	}

	return nil
}

func (r *Repository) GetTransferRequestResult(id uint) error {
	url := "http://127.0.0.1:4000/async/calculate_success"

	authKey := "secret-async-orbits"

	requestBody := map[string]interface{}{"id": int(id)}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", authKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *Repository) SetTransferRequestResult(id int, status bool) error {
	result := &ds.TransferRequest{ID: uint(id)}
	err := r.db.Model(result).Update("result", status).Omit("result").Error
	if err != nil {
		return err
	}
	return nil
}

// =================================================================================
// ---------------------------------------------------------------------------------
// ------------------------- TRANSFERS_TO_ORBITS METHODS ---------------------------
// ---------------------------------------------------------------------------------

// новое создание м-м
func (r *Repository) CreateTransferToOrbit(transfer_to_orbit ds.TransferToOrbit) error {
	return r.db.Create(&transfer_to_orbit).Error
}

func (r *Repository) AddTransferToOrbits(orbit_refer, request_refer uint) error {
	orbit := ds.Orbit{ID: orbit_refer}
	request := ds.TransferRequest{ID: request_refer}
	var currTransfers []ds.TransferToOrbit

	err := r.db.Where("request_refer = ?", request_refer).Find(&currTransfers).Error
	if err != nil {
		return err
	}

	err = r.db.Where("request_refer = ?", request_refer).Where("orbit_refer = ?", orbit_refer).First(&ds.TransferToOrbit{}).Error
	if err != nil {
		NewMtM := &ds.TransferToOrbit{
			Orbit:         orbit,
			OrbitRefer:    orbit_refer,
			Request:       request,
			RequestRefer:  request_refer,
			TransferOrder: uint(len(currTransfers) + 1),
		}
		return r.db.Create(NewMtM).Error
	} else {
		return err
	}
}

//func (r *Repository) AddTransferToOrbits(request_refer, orbit_refer uint) error {
//	orbit := ds.Orbit{ID: orbit_refer}
//	request := ds.TransferRequest{ID: request_refer}
//	var currTransfers []ds.TransferToOrbit
//
//	err := r.db.Where("request_refer = ?", request_refer).Find(&currTransfers).Error
//	if err != nil {
//		return err
//	}
//
//	err = r.db.Where("request_refer = ?", request_refer).Where("orbit_refer = ?", orbit_refer).First(&ds.TransferToOrbit{}).Error
//	if err != nil {
//		NewMtM := &ds.TransferToOrbit{
//			Orbit:         orbit,
//			OrbitRefer:    orbit_refer,
//			Request:       request,
//			RequestRefer:  request_refer,
//			TransferOrder: uint(len(currTransfers) + 1),
//		}
//		return r.db.Create(NewMtM).Error
//	} else {
//		return err
//	}
//}

func (r *Repository) GetOrbitOrder(id int) ([]ds.OrbitOrder, error) {
	transfer_to_orbits := []ds.TransferToOrbit{}

	err := r.db.Model(&ds.TransferToOrbit{}).Where("request_refer = ?", id).Find(&transfer_to_orbits).Error
	if err != nil {
		return []ds.OrbitOrder{}, err
	}

	var orbitOrders []ds.OrbitOrder
	for _, transfer_to_orbit := range transfer_to_orbits {
		orbit, err := r.GetOrbitByID(transfer_to_orbit.OrbitRefer)
		if err != nil {
			continue
		}
		orbitOrder := ds.OrbitOrder{
			OrbitName:     orbit,
			TransferOrder: int(transfer_to_orbit.TransferOrder),
		}
		orbitOrders = append(orbitOrders, orbitOrder)
	}

	return orbitOrders, nil
}

// обновление порядка перелетов в м-м
func (r *Repository) UpdateTransferOrders(updateRequest ds.UpdateTransferOrdersBody) error {

	for orbitName, transferOrder := range updateRequest.TransferOrder {
		orbit, err := r.GetOrbitByName(orbitName)
		if err != nil {
			return err
		}

		err = r.db.Model(&ds.TransferToOrbit{}).Where("orbit_refer = ?", orbit.ID).
			Updates(map[string]interface{}{"transfer_order": transferOrder}).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// удаляет одну запись за раз
func (r *Repository) DeleteTransferToOrbitSingle(transfer_id string, orbit_id int) error {
	tx := r.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// ошибка если что-то не совпадает/чего-то нет
	if tx.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil ||
		tx.Where("request_refer = ?", transfer_id).First(&ds.TransferToOrbit{}).Error != nil {

		tx.Rollback()
		return tx.Error
	}

	var currTransfer ds.TransferToOrbit

	// получение удаляемой записи из м-м
	if err := tx.Where("request_refer = ?", transfer_id).
		Where("orbit_refer = ?", orbit_id).First(&currTransfer).Error; err != nil {
		tx.Rollback()
		return err
	}

	// обновление transfer_order для записей, у которых он больше чем у удаляемой
	if err := tx.Model(&ds.TransferToOrbit{}).
		Where("request_refer = ?", transfer_id).
		Where("transfer_order > ?", currTransfer.TransferOrder).
		Update("transfer_order", gorm.Expr("transfer_order - 1")).Error; err != nil {
		tx.Rollback()
		return err
	}

	// удаление указанной м-м
	if err := tx.Where("request_refer = ?", transfer_id).
		Where("orbit_refer = ?", orbit_id).
		Delete(&ds.TransferToOrbit{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Завершение транзакции
	return tx.Commit().Error
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
