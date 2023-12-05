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
	Login    string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Username    string
	Role        role.Role
	ExpiresIn   int    `json:"expires_in"`
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type registerReq struct {
	Name string `json:"name"` // лучше назвать то же самое что login
	Pass string `json:"pass"`
}

type registerResp struct {
	Ok bool `json:"ok"`
}

type jsonMap map[string]string

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
		clientMethods.POST("/orbits/:orbit_name/add", a.addOrbitToRequest)
		clientMethods.PUT("/transfer_requests/:req_id/client_change_status", a.clientChangeTransferRequestStatus)
		clientMethods.POST("/transfer_requests/:req_id/delete", a.deleteTransferRequest)
		clientMethods.DELETE("/transfer_to_orbit/delete_single", a.deleteTransferToOrbitSingle)
	}

	moderMethods := a.r.Group("", a.WithAuthCheck(role.Moderator))
	{
		moderMethods.PUT("/orbits/:orbit_name/edit", a.editOrbit)
		moderMethods.POST("/orbits/new_orbit", a.newOrbit)
		moderMethods.DELETE("/orbits/change_status/:orbit_name", a.changeOrbitStatus)
		moderMethods.PUT("/transfer_requests/:req_id/moder_change_status", a.moderChangeTransferRequestStatus)
		moderMethods.GET("/ping", a.ping)
	}

	authorizedMethods := a.r.Group("", a.WithAuthCheck(role.Client, role.Moderator))
	{
		authorizedMethods.GET("/transfer_requests", a.getAllRequests)
		authorizedMethods.GET("/transfer_requests/:req_id", a.getDetailedRequest)
		authorizedMethods.GET("/transfer_requests/status/:status", a.getRequestsByStatus)
	}

	a.r.Run(":8000")

	log.Println("Server is down")
}

// @Summary Получение всех орбит со статусом "Доступна"
// @Description Возвращает всех доступные орбиты
// @Tags Орбиты
// @Accept json
// @Produce json
// @Success 302 {} json
// @Param orbit_name query string false "Название орбиты или его часть"
// @Router /orbits [get]
func (a *Application) getAllOrbits(c *gin.Context) {
	orbitName := c.Query("orbit_name")
	orbitIncl := c.Query("orbit_incl")
	isCircle := c.Query("is_circle")

	allOrbits, err := a.repo.GetAllOrbits(orbitName, orbitIncl, isCircle)

	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusFound, allOrbits)

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
		c.Error(err)
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

// фактическ - удаление услуги (status=false, не выводится)
func (a *Application) changeOrbitStatus(c *gin.Context) {
	orbitName := c.Param("orbit_name")

	err := a.repo.ChangeOrbitStatus(orbitName)

	if err != nil {
		c.Error(err)
		return
	}

	//c.Redirect(http.StatusFound, "/orbits")
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
// @Success      302  {object}  string
// @Param Body body jsonMap true "Данные заказа"
// @Router       /orbits/{orbit_name}/add [post]
func (a *Application) addOrbitToRequest(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	// Получение инфы об орбите -> orbit.ID
	orbit, err := a.repo.GetOrbitByName(orbit_name)
	if err != nil {
		c.Error(err)
		return
	}

	var requestData jsonMap

	if err = c.BindJSON(&requestData); err != nil {
		c.Error(err)
		return
	}

	log.Println("c_name: ", requestData)

	request := &ds.TransferRequest{}
	request, err = a.repo.CreateTransferRequest(requestData["client_name"])
	if err != nil {
		c.Error(err)
		return
	}

	err = a.repo.AddTransferToOrbits(orbit.ID, request.ID)
	if err != nil {
		c.Error(err)
		return
	}
}

// @Summary      Получение всех заявок на трансфер
// @Description  Получает все заявки на трансфер
// @Tags         Заявки на трансфер
// @Produce      json
// @Success      302  {object}  string
// @Router       /transfer_requests [get]
func (a *Application) getAllRequests(c *gin.Context) {
	dateStart := c.Query("date_start")
	dateFin := c.Query("date_fin")

	userRole, exists := c.Get("role")
	if !exists {
		panic(exists)
	}
	//userUUID, exists := c.Get("userUUID")
	//if !exists {
	//	panic(exists)
	//}

	requests, err := a.repo.GetAllRequests(userRole, dateStart, dateFin)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusFound, requests)
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
		// ... handle error
		panic(err)
	}

	requests, err := a.repo.GetRequestByID(uint(req_id))
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusFound, requests)
}

// надо??
func (a *Application) getRequestsByStatus(c *gin.Context) {
	req_status := c.Param("req_status")

	requests, err := a.repo.GetRequestsByStatus(req_status)
	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusFound, requests)
}

// TransferID не нужен в текущей ситуации (/transfer_requests/:req_id/<>statusChange), т.к. можно получать его из урл
// Оба метода почти идентичны -> сделать один большой = лучше?
// @Summary      Изменение статуса заявки на трансфер (для модератора)
// @Description  Изменяет статус заявки на трансфер на любой из доступных для модератора
// @Tags         Заявки на трансфер
// @Accept json
// @Produce      json
// @Success      201  {object}  string
// @Param request body ds.ChangeTransferStatusRequestBody true "Данные о заявке"
// @Router /transfer_requests/{transferID}/moder_change_status [put]
func (a *Application) moderChangeTransferRequestStatus(c *gin.Context) {
	var requestBody ds.ChangeTransferStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	currRequest, err := a.repo.GetRequestByID(requestBody.TransferID)
	if err != nil {
		c.Error(err)
		return
	}

	currUser, err := a.repo.GetUserByName(requestBody.UserName)
	if err != nil {
		c.Error(err)
		return
	}

	if !slices.Contains(ds.ReqStatuses, requestBody.Status) {
		c.String(http.StatusBadRequest, "Неверный статус")
		return
	}

	if currRequest.ModerRefer == currUser.UUID {
		if slices.Contains(ds.ReqStatuses[len(ds.ReqStatuses)-3:], requestBody.Status) {
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

// надо ли делать проверку является ли пользователь клиентом?
// @Summary      Изменение статуса заявки на трансфер (для клиента)
// @Description  Изменяет статус заявки на трансфер на любой из доступных для клиента
// @Tags         Заявки на трансфер
// @Accept json
// @Produce      json
// @Param request body ds.ChangeTransferStatusRequestBody true "Данные о заявке"
// @Success      201  {object}  string
// @Router /transfer_requests/{transferID}/client_change_status [put]
func (a *Application) clientChangeTransferRequestStatus(c *gin.Context) {
	var requestBody ds.ChangeTransferStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	currRequest, err := a.repo.GetRequestByID(requestBody.TransferID)
	if err != nil {
		c.Error(err)
		return
	}

	currUser, err := a.repo.GetUserByName(requestBody.UserName)
	if err != nil {
		c.Error(err)
		return
	}

	if !slices.Contains(ds.ReqStatuses, requestBody.Status) {
		c.String(http.StatusBadRequest, "Неверный статус")
		return
	}

	if currRequest.ClientRefer == currUser.UUID {
		if slices.Contains(ds.ReqStatuses[:2], requestBody.Status) {
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
}

// @Summary      Логическое удаление заявки на трансфер
// @Description  Изменяет статус заявки на трансфер на "Удалена"
// @Tags         Заявки на трансфер
// @Produce      json
// @Success      302  {object}  string
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

	c.String(http.StatusFound, "TransferRequest & TransferToOrbit were deleted")
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

	if req.Pass == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("pass is empty"))
		return
	}

	if req.Name == "" {
		c.AbortWithError(http.StatusBadRequest, fmt.Errorf("name is empty"))
		return
	}

	err = a.repo.Register(&ds.User{
		UUID: uuid.New(),
		Role: role.Client,
		Name: req.Name,
		Pass: a.repo.GenerateHashString(req.Pass), // пароли делаем в хешированном виде и далее будем сравнивать хеши, чтобы их не угнали с базой вместе
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
	//log.Println("---JSON--- ", req.Login, " --- ", req.Password)

	user, err := a.repo.GetUserByName(req.Login)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	if req.Login == user.Name && user.Pass == a.repo.GenerateHashString(req.Password) {
		// значит проверка пройдена
		log.Println("проверка пройдена")
		// генерируем ему jwt
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
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("token is nil"))

			return
		}

		strToken, err := token.SignedString([]byte(cfg.JWT.Token))
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("cant create str token"))

			return
		}

		//httpOnly=true, secure=true -> не могу читать куки на фронте ...
		c.SetCookie("orbits-api-token", "Bearer "+strToken, int(time.Now().Add(time.Second*3600).
			Unix()), "", "", false, false)

		c.JSON(http.StatusOK, loginResp{
			Username:    user.Name,
			Role:        user.Role,
			AccessToken: strToken,
			TokenType:   "Bearer",
			ExpiresIn:   int(cfg.JWT.ExpiresIn.Seconds()),
		})
		log.Println("\nUSER: ", user.Name, "\n", strToken, "\n")
		c.AbortWithStatus(http.StatusOK)
	} else {
		c.AbortWithStatus(http.StatusForbidden) // отдаем 403 ответ в знак того что доступ запрещен
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
	// получаем заголовок
	jwtStr, err := GetJWTToken(c)
	if err != nil {
		panic(err)
	}

	if !strings.HasPrefix(jwtStr, jwtPrefix) { // если нет префикса то нас дурят!
		c.AbortWithStatus(http.StatusForbidden) // отдаем что нет доступа

		return // завершаем обработку
	}

	// отрезаем префикс
	jwtStr = jwtStr[len(jwtPrefix):]

	_, err = jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWT.Token), nil
	})
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		log.Println(err)

		return
	}

	// сохраняем в блеклист редиса
	err = a.redis.WriteJWTToBlackList(c.Request.Context(), jwtStr, a.config.JWT.ExpiresIn)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)

		return
	}

	c.Status(http.StatusOK)
}
