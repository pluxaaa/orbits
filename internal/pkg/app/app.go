package app

import (
	"L1/internal/app/ds"
	"L1/internal/app/dsn"
	"L1/internal/app/repository"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"slices"
	"strconv"
)

type Application struct {
	repo repository.Repository
	r    *gin.Engine
}

func New() Application {
	app := Application{}

	repo, _ := repository.New(dsn.FromEnv())

	app.repo = *repo

	return app

}

func (a *Application) StartServer() {
	log.Println("Server started")

	a.r = gin.Default()

	a.r.LoadHTMLGlob("templates/*.html")
	a.r.Static("/css", "./templates")

	a.r.GET("orbits", a.getAllOrbits)
	a.r.GET("orbits/:orbit_name", a.getDetailedOrbit)
	a.r.PUT("orbits/:orbit_name/edit", a.editOrbit)
	a.r.POST("orbits/new_orbit", a.newOrbit)
	a.r.POST("orbits/:orbit_name/add", a.addOrbitToRequest)
	a.r.DELETE("orbits/change_status/:orbit_name", a.changeOrbitStatus)
	//a.r.DELETE("orbits/change_status/:orbit_name", a.deleteOrbit)

	a.r.GET("transfer_requests", a.getAllRequests)
	a.r.GET("transfer_requests/:req_id", a.getDetailedRequest)
	a.r.GET("transfer_requests/status/:status", a.getRequestsByStatus)
	a.r.PUT("transfer_requests/:req_id/moder_change_status", a.moderChangeTransferRequestStatus)
	a.r.PUT("transfer_requests/:req_id/client_change_status", a.clientChangeTransferRequestStatus)
	a.r.POST("transfer_requests/:req_id/delete", a.deleteTransferRequest)

	a.r.DELETE("/transfer_to_orbit/delete_single", a.deleteTransferToOrbitSingle)

	a.r.Run(":8000")

	log.Println("Server is down")
}

func (a *Application) getAllOrbits(c *gin.Context) {
	orbitName := c.Query("orbit_name")

	allOrbits, err := a.repo.GetAllOrbits(orbitName)

	if err != nil {
		c.Error(err)
	}

	c.JSON(http.StatusFound, allOrbits)

}

func (a *Application) getDetailedOrbit(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	//if orbit_name == "favicon.ico" {
	//	return
	//}

	orbit, err := a.repo.GetOrbitByName(orbit_name)
	if err != nil {
		c.Error(err)
		return
	}
	//c.HTML(http.StatusOK, "orbitDetail.html", gin.H{
	//	"Name":        orbit.Name,
	//	"IsAvailable": orbit.IsAvailable,
	//	"Apogee":      orbit.Apogee,
	//	"Perigee":     orbit.Perigee,
	//	"Inclination": orbit.Inclination,
	//	"Description": orbit.Description,
	//	"ImageURL":    orbit.ImageURL,
	//})

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

	c.JSON(http.StatusOK, gin.H{
		"ID":          requestBody.ID,
		"Name":        requestBody.Name,
		"Apogee":      requestBody.Apogee,
		"Perigee":     requestBody.Perigee,
		"Inclination": requestBody.Inclination,
		"Description": requestBody.Description,
		"ImageURL":    requestBody.ImageURL,
	})
}

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

	c.JSON(http.StatusOK, gin.H{
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

// не используется -> физ удаление орбиты
//func (a *Application) deleteOrbit(c *gin.Context) {
//	orbit_name := c.Param("orbit_name")
//
//	orbit, err := a.repo.GetOrbitByName(orbit_name)
//	if err != nil {
//		c.Error(err)
//		return
//	}
//
//	err = a.repo.DeleteOrbit(orbit.ID)
//	if err != nil {
//		c.Error(err)
//		return
//	}
//}

// в json надо послать айди клиента
func (a *Application) addOrbitToRequest(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	//получение инфы об орбите -> orbit.ID
	orbit, err := a.repo.GetOrbitByName(orbit_name)
	if err != nil {
		c.Error(err)
		return
	}
	// вместо структуры для json использую map
	// map: key-value
	// jsonMap: string-int
	// можно использовать string-interface{} (определяемый тип, в данном случае - пустой)
	// тогда будет jsonMap["client_id"].int
	var jsonMap map[string]int

	if err = c.BindJSON(&jsonMap); err != nil {
		c.Error(err)
		return
	}
	log.Println("c_id: ", jsonMap)

	request := &ds.TransferRequest{}
	request, err = a.repo.CreateTransferRequest(uint(jsonMap["client_id"]))
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

func (a *Application) getAllRequests(c *gin.Context) {
	requests, err := a.repo.GetAllRequests()

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusFound, requests)
}

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

	if *currUser.IsModer != true {
		c.String(http.StatusForbidden, "У пользователя должна быть роль модератора")
		return
	} else {
		if currRequest.ModerRefer == currUser.ID {
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
}

// надо ли делать проверку является ли пользователь клиентом?
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

	if *currUser.IsModer == true {
		c.String(http.StatusForbidden, "У пользователя должна быть роль клиента")
		return
	} else {
		if currRequest.ClientRefer == currUser.ID {
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
}

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

	c.String(http.StatusCreated, "TransferRequest & TransferToOrbit were deleted")
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
