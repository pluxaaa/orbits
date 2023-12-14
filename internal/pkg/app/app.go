package app

import (
	"L1/docs"
	"L1/internal/app/config"
	"L1/internal/app/ds"
	"L1/internal/app/dsn"
	"L1/internal/app/redis"
	"L1/internal/app/repository"
	"L1/internal/app/role"
	"context"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"strings"

	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"

	"encoding/json"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"
)

type Application struct {
	repo   *repository.Repository
	r      *gin.Engine
	config *config.Config
	redis  *redis.Client
}

type loginReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type loginResp struct {
	Login       string `json:"login"`
	Role        int    `json:"role"`
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type registerReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type registerResp struct {
	Ok bool `json:"ok"`
}

func New(ctx context.Context) (*Application, error) {
	cfg, err := config.NewConfig(ctx)
	if err != nil {
		return nil, err
	}

	repo, err := repository.New(dsn.FromEnv())
	if err != nil {
		return nil, err
	}

	redisClient, err := redis.New(ctx, cfg.Redis)
	if err != nil {
		return nil, err
	}

	return &Application{
		config: cfg,
		repo:   repo,
		redis:  redisClient,
	}, nil
}

func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	docs.SwaggerInfo.BasePath = "/"
	a.r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	a.r.POST("/login", a.login)
	a.r.POST("/register", a.register)
	a.r.POST("/logout", a.logout)

	a.r.GET("/orbits", a.getAllOrbits)
	a.r.GET("/orbits/:orbit_name", a.getDetailedOrbit)

	clientMethods := a.r.Group("", a.WithAuthCheck(role.Client))
	{
		//старое создание заявки + добавление в м-м (не используется скорее всего) -> удалить?
		//clientMethods.POST("/orbits/:orbit_name/add", a.addOrbitToRequest)

		//актуальное создание заявки + добавление в м-м
		clientMethods.POST("/transfer_requests/create", a.createTransferRequest)

		//актуальное обновление записей в м-м
		clientMethods.PUT("/transfer_requests/set_orbits", a.setRequestOrbits)

		clientMethods.POST("/transfer_requests/:req_id/delete", a.deleteTransferRequest)
		clientMethods.DELETE("/transfer_to_orbit/delete_single", a.deleteTransferToOrbitSingle)
	}

	moderMethods := a.r.Group("", a.WithAuthCheck(role.Moderator))
	{
		moderMethods.PUT("/orbits/:orbit_name/edit", a.editOrbit)
		moderMethods.POST("/orbits/new_orbit", a.newOrbit)
		moderMethods.DELETE("/orbits/change_status/:orbit_name", a.changeOrbitStatus)
		moderMethods.GET("/ping", a.ping)
	}

	authorizedMethods := a.r.Group("", a.WithAuthCheck(role.Client, role.Moderator))
	{
		authorizedMethods.GET("/transfer_requests", a.getAllRequests)
		authorizedMethods.GET("/transfer_requests/:req_id", a.getDetailedRequest)
		authorizedMethods.GET("/transfer_requests/status/:status", a.getRequestsByStatus)
		authorizedMethods.GET("/transfer_to_orbit/:req_id", a.getOrbitsFromTransfer)
		authorizedMethods.PUT("/transfer_requests/change_status", a.changeRequestStatus)
	}

	a.r.Run(":8000")

	log.Println("Server is down")
}

// @Summary Получение всех орбит со статусом "Доступна"
// @Description Возвращает всех доступные орбиты
// @Tags Орбиты
// @Accept json
// @Produce json
// @Success 200 {} json
// @Param orbit_name query string false "Название орбиты или его часть"
// @Router /orbits [get]
func (a *Application) getAllOrbits(c *gin.Context) {
	orbitName := c.Query("orbit_name")
	orbitIncl := c.Query("orbit_incl")
	isCircle := c.Query("is_circle")

	allOrbits, err := a.repo.GetAllOrbits(orbitName, orbitIncl, isCircle)

	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, allOrbits)

}

// @Summary      Получение детализированной информации об орбите
// @Description  Возвращает подробную информацию об орбите по ее названию
// @Tags         Орбиты
// @Produce      json
// @Param orbit_name path string true "Название орбиты"
// @Success      200  {object}  string
// @Router       /orbits/{orbit_name} [get]
func (a *Application) getDetailedOrbit(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	orbit, err := a.repo.GetOrbitByName(orbit_name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Name":        orbit.Name,
		"IsAvailable": orbit.IsAvailable,
		"Apogee":      orbit.Apogee,
		"Perigee":     orbit.Perigee,
		"Inclination": orbit.Inclination,
		"Description": orbit.Description,
		"ImageURL":    orbit.ImageURL,
	})

}

// фактически - удаление услуги (status=false, не выводится)
func (a *Application) changeOrbitStatus(c *gin.Context) {
	orbitName := c.Param("orbit_name")

	err := a.repo.ChangeOrbitStatus(orbitName)

	if err != nil {
		c.Error(err)
		return
	}
}

// @Summary      Добавление новой орбиты
// @Description  Добавляет орбиту с полями, указанныим в JSON
// @Tags Орбиты
// @Accept json
// @Produce      json
// @Param orbit body ds.Orbit true "Данные новой орбиты"
// @Success      201  {object}  string
// @Router       /orbits/new_orbit [post]
func (a *Application) newOrbit(c *gin.Context) {
	var requestBody ds.Orbit

	if err := c.BindJSON(&requestBody); err != nil {
		log.Println("ERROR")
		c.Error(err)
	}

	err := a.repo.AddOrbit(&requestBody, requestBody.ImageURL)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"ID":          requestBody.ID,
		"Name":        requestBody.Name,
		"Apogee":      requestBody.Apogee,
		"Perigee":     requestBody.Perigee,
		"Inclination": requestBody.Inclination,
		"Description": requestBody.Description,
		"ImageURL":    requestBody.ImageURL,
	})
}

// @Summary      Изменение орбиты
// @Description  Обновляет данные об орбите, основываясь на полях из JSON
// @Tags         Орбиты
// @Accept 		 json
// @Produce      json
// @Param orbit body ds.Orbit false "Орбита"
// @Success      201  {object}  string
// @Router       /orbits/{orbit_name}/edit [put]
func (a *Application) editOrbit(c *gin.Context) {
	orbit_name := c.Param("orbit_name")
	orbit, err := a.repo.GetOrbitByName(orbit_name)

	var editingOrbit ds.Orbit

	if err := c.BindJSON(&editingOrbit); err != nil {
		c.Error(err)
	}

	err = a.repo.EditOrbit(orbit.ID, editingOrbit)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"ID":          editingOrbit.ID,
		"Name":        editingOrbit.Name,
		"IsAvailable": editingOrbit.IsAvailable,
		"Apogee":      editingOrbit.Apogee,
		"Perigee":     editingOrbit.Perigee,
		"Inclination": editingOrbit.Inclination,
		"Description": editingOrbit.Description,
		"ImageURL":    editingOrbit.ImageURL,
	})
}

// @Summary      Добавление орбиты в заявку на трансфер
// @Description  Создает заявку на трансфер в статусе (или добавляет в открытую) и добавляет выбранную орбиту
// @Tags Общее
// @Accept json
// @Produce      json
// @Success      200  {object}  string
// @Param Body body jsonMap true "Данные заказа"
// @Router       /orbits/{orbit_name}/add [post]
// удалить?
// удалить?
//func (a *Application) addOrbitToRequest(c *gin.Context) {
//	orbit_name := c.Param("orbit_name")
//
//	// Получение инфы об орбите -> orbit.ID
//	orbit, err := a.repo.GetOrbitByName(orbit_name)
//	if err != nil {
//		c.Error(err)
//		return
//	}
//
//	userUUID, exists := c.Get("userUUID")
//	if !exists {
//		panic(exists)
//	}
//
//	request := &ds.TransferRequest{}
//	request, err = a.repo.CreateTransferRequest(userUUID.(uuid.UUID))
//	if err != nil {
//		c.Error(err)
//		return
//	}
//
//	err = a.repo.AddTransferToOrbits(orbit.ID, request.ID)
//	if err != nil {
//		c.Error(err)
//		return
//	}
//}

func (a *Application) createTransferRequest(c *gin.Context) {
	var request_body ds.CreateTransferRequestBody

	if err := c.BindJSON(&request_body); err != nil {
		c.String(http.StatusBadGateway, "Не могу распознать json")
		return
	}

	_userUUID, ok := c.Get("userUUID")

	if !ok {
		c.String(http.StatusInternalServerError, "Вы сначала должны залогиниться")
		return
	}

	userUUID := _userUUID.(uuid.UUID)
	reqID, err := a.repo.CreateTransferRequest(request_body, userUUID)

	if err != nil {
		c.Error(err)
		c.String(http.StatusNotFound, "Не могу добавить орбиту")
		return
	}

	c.JSON(http.StatusCreated, reqID)
}

func (a *Application) setRequestOrbits(c *gin.Context) {
	var requestBody ds.SetRequestOrbitsRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.String(http.StatusBadRequest, "Не получается распознать json запрос")
		return
	}

	err := a.repo.SetRequestOrbits(requestBody.RequestID, requestBody.Orbits)
	if err != nil {
		c.String(http.StatusInternalServerError, "Не получилось задать регионы для заявки\n"+err.Error())
	}

	c.String(http.StatusCreated, "Регионы заявки успешно заданы!")

}

// @Summary      Получение всех заявок на трансфер
// @Description  Получает все заявки на трансфер
// @Tags         Заявки на трансфер
// @Produce      json
// @Success      200  {object}  string
// @Router       /transfer_requests [get]
func (a *Application) getAllRequests(c *gin.Context) {
	dateStart := c.Query("date_start")
	dateFin := c.Query("date_fin")
	status := c.Query("status")
	log.Println(status)

	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}
	//userUUID, exists := c.Get("userUUID")
	//if !exists {
	//	panic(exists)
	//}

	requests, err := a.repo.GetAllRequests(userRole, dateStart, dateFin, status)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, requests)
}

// @Summary      Получение детализированной заявки на трансфер
// @Description  Получает подробную информаицю о заявке на трансфер
// @Tags         Заявки на трансфер
// @Produce      json
// @Param req_id path string true "ID заявки"
// @Success      301  {object}  string
// @Router       /transfer_requests/{req_id} [get]
func (a *Application) getDetailedRequest(c *gin.Context) {
	req_id, err := strconv.Atoi(c.Param("req_id"))
	if err != nil {
		log.Println("REQ ID: ", req_id)
		panic(err)
	}

	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}
	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}

	request, err := a.repo.GetRequestByID(uint(req_id), userUUID.(uuid.UUID), userRole)
	if err != nil {
		c.JSON(http.StatusForbidden, err)
		return
	}

	c.JSON(http.StatusOK, request)
}

// надо??
func (a *Application) getRequestsByStatus(c *gin.Context) {
	req_status := c.Param("req_status")

	requests, err := a.repo.GetRequestsByStatus(req_status)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (a *Application) changeRequestStatus(c *gin.Context) {
	var requestBody ds.ChangeTransferStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}
	log.Println(requestBody)

	userRole, exists := c.Get("userRole")
	if !exists {
		panic(exists)
	}
	userUUID, exists := c.Get("userUUID")
	if !exists {
		panic(exists)
	}

	currRequest, err := a.repo.GetRequestByID(requestBody.TransferID, userUUID.(uuid.UUID), userRole)
	if err != nil {
		c.AbortWithError(http.StatusForbidden, err)
		return
	}

	if !slices.Contains(ds.ReqStatuses, requestBody.Status) {
		c.String(http.StatusBadRequest, "Неверный статус")
		return
	}

	if userRole == role.Client {
		if currRequest.ClientRefer == userUUID {
			if slices.Contains(ds.ReqStatuses[:3], requestBody.Status) {
				if currRequest.Status != ds.ReqStatuses[0] {
					c.String(http.StatusBadRequest, "Нельзя поменять статус с ", currRequest.Status,
						" на ", requestBody.Status)
					return
				}
				err = a.repo.ChangeRequestStatus(requestBody.TransferID, requestBody.Status)

				if err != nil {
					c.Error(err)
					return
				}

				c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
				return
			} else {
				c.String(http.StatusForbidden, "Клиент не может установить статус ", requestBody.Status)
				return
			}
		} else {
			c.String(http.StatusForbidden, "Клиент не является ответственным")
			return
		}
	} else {
		if currRequest.ModerRefer == userUUID {
			if slices.Contains(ds.ReqStatuses[len(ds.ReqStatuses)-2:], requestBody.Status) {
				err = a.repo.ChangeRequestStatus(requestBody.TransferID, requestBody.Status)

				if err != nil {
					c.Error(err)
					return
				}

				c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
				return
			} else {
				c.String(http.StatusForbidden, "Модератор не может установить статус ", requestBody.Status)
				return
			}
		} else {
			c.String(http.StatusForbidden, "Модератор не является ответственным")
			return
		}
	}
}

// @Summary      Логическое удаление заявки на трансфер
// @Description  Изменяет статус заявки на трансфер на "Удалена"
// @Tags         Заявки на трансфер
// @Produce      json
// @Success      200  {object}  string
// @Param req_id path string true "ID заявки"
// @Router /transfer_requests/{req_id}/delete [post]
func (a *Application) deleteTransferRequest(c *gin.Context) {
	req_id, err1 := strconv.Atoi(c.Param("req_id"))
	if err1 != nil {
		// ... handle error
		panic(err1)
	}

	err1, err2 := a.repo.DeleteTransferRequest(uint(req_id)), a.repo.DeleteTransferToOrbitEvery(uint(req_id))

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusOK, "TransferRequest & TransferToOrbit were deleted")
}

func (a *Application) getOrbitsFromTransfer(c *gin.Context) { // нужно добавить проверку на авторизацию пользователя
	req_id, err := strconv.Atoi(c.Param("req_id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Ошибка в ID заявки")
		return
	}

	orbits, err := a.repo.GetOrbitsFromTransfer(req_id)
	log.Println(orbits)
	if err != nil {
		c.String(http.StatusInternalServerError, "Ошибка при получении орбит из заявки")
		return
	}

	c.JSON(http.StatusOK, orbits)

}

// удаление записи (одной) из м-м по двум айди
func (a *Application) deleteTransferToOrbitSingle(c *gin.Context) {
	var requestBody ds.TransferToOrbit

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	err1, err2 := a.repo.DeleteTransferToOrbitSingle(requestBody.RequestRefer, requestBody.OrbitRefer)

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusCreated, "Transfer-to-Orbit m-m was deleted")
}

func (a *Application) ping(c *gin.Context) {
	log.Println("ping func")
	c.JSON(http.StatusOK, gin.H{
		"auth": true,
	})
}

// @Summary Зарегистрировать нового пользователя
// @Description Добавляет в БД нового пользователя
// @Tags Аутентификация
// @Produce json
// @Accept json
// @Success 200 {object} registerResp
// @Param request_body body registerReq true "Данные для регистрации"
// @Router /register [post]
func (a *Application) register(c *gin.Context) {
	req := &registerReq{}

	err := json.NewDecoder(c.Request.Body).Decode(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if req.Password == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("pass is empty"))
		return
	}

	if req.Login == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("name is empty"))
		return
	}

	err = a.repo.Register(&ds.User{
		UUID: uuid.New(),
		Role: role.Client,
		Name: req.Login,
		Pass: a.repo.GenerateHashString(req.Password), // пароли делаем в хешированном виде и далее будем сравнивать хеши, чтобы их не угнали с базой вместе
	})
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, &registerResp{
		Ok: true,
	})
}

// @Summary Вход в систему
// @Description Проверяет данные для входа и в случае успеха возвращает токен для входа
// @Tags Аутентификация
// @Produce json
// @Accept json
// @Success 200 {object} loginResp
// @Param request_body body loginReq true "Данные для входа"
// @Router /login [post]
func (a *Application) login(c *gin.Context) {
	cfg := a.config
	req := &loginReq{}
	err := json.NewDecoder(c.Request.Body).Decode(req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)

		return
	}

	user, err := a.repo.GetUserByName(req.Login)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	if req.Login == user.Name && user.Pass == a.repo.GenerateHashString(req.Password) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, &ds.JWTClaims{
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: time.Now().Add(time.Second * 3600).Unix(), //1h
				IssuedAt:  time.Now().Unix(),
				Issuer:    "web-admin",
			},
			UserUUID: user.UUID,
			Role:     user.Role,
		})

		if token == nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Токен = nil"))

			return
		}

		strToken, err := token.SignedString([]byte(cfg.JWT.Token))
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("Невозможно получить строку из токена"))

			return
		}

		//httpOnly=true, secure=true -> не могу читать куки на фронте ...
		c.SetCookie("orbits-api-token", "Bearer "+strToken, int(time.Now().Add(time.Second*3600).
			Unix()), "", "", true, true)

		c.JSON(http.StatusOK, loginResp{
			Login:       user.Name,
			Role:        int(user.Role),
			AccessToken: strToken,
			TokenType:   "Bearer",
			ExpiresIn:   int(cfg.JWT.ExpiresIn.Seconds()),
		})
		log.Println("\nUSER: ", user.Name, "\n", strToken, "\n")
		c.AbortWithStatus(http.StatusOK)
	} else {
		c.AbortWithStatus(http.StatusForbidden)
	}
}

// @Summary Выйти из системы
// @Details Деактивирует текущий токен пользователя, добавляя его в блэклист в редисе
// @Tags Аутентификация
// @Produce json
// @Accept json
// @Success 200
// @Router /logout [post]
func (a *Application) logout(c *gin.Context) {
	jwtStr, err := GetJWTToken(c)
	if err != nil {
		panic(err)
	}

	if !strings.HasPrefix(jwtStr, jwtPrefix) {
		c.AbortWithStatus(http.StatusForbidden)

		return
	}

	jwtStr = jwtStr[len(jwtPrefix):]

	_, err = jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWT.Token), nil
	})
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Println(err)

		return
	}

	err = a.redis.WriteJWTToBlackList(c.Request.Context(), jwtStr, a.config.JWT.ExpiresIn)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	c.Status(http.StatusOK)
}
