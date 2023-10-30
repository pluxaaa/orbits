package app

import (
	"L1/internal/app/ds"
	"L1/internal/app/dsn"
	"L1/internal/app/repository"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
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
	a.r.GET("transfers", a.getAllRequests)
	a.r.GET("transfers/id/:req_id", a.getDetailedRequest)
	a.r.GET("transfers/status/:status", a.getRequestsByStatus)

	a.r.PUT("orbits/:orbit_name/edit", a.editOrbit)
	a.r.PUT("orbits/add", a.newOrbit)
	a.r.PUT("transfers/:req_id/moder_change_status", a.moderChangeTransferStatus)
	a.r.PUT("transfers/:req_id/client_change_status", a.clientChangeTransferStatus)

	a.r.POST("/change_status/:orbit_name", a.changeOrbitStatus)
	a.r.POST("/:orbit_name/add", a.addOrbitToRequest)

	a.r.DELETE("/transfer_to_orbit/delete", a.deleteTransferToOrbit)

	a.r.Run(":8000")

	log.Println("Server is down")
}

func (a *Application) getAllOrbits(c *gin.Context) {
	orbitName := c.Query("orbit_name")

	if orbitName == "" {
		log.Println("ALL ORBITS 1")

		allOrbits, err := a.repo.GetAllOrbits()

		if err != nil {
			c.Error(err)
		}

		//для лаб3 нужен хтмл
		//c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
		//	"orbits": a.repo.FilterOrbits(allOrbits),
		//})

		//для лаб4 нужен жсон
		c.JSON(http.StatusOK, gin.H{
			"orbits": a.repo.FilterOrbits(allOrbits),
		})
	} else {
		log.Println("!!! SEARCHING ORBITS !!!")

		foundOrbits, err := a.repo.SearchOrbits(orbitName)
		if err != nil {
			c.Error(err)
			return
		}

		//для лаб3 нужен хтмл
		//c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
		//	"orbits":    a.repo.FilterOrbits(foundOrbits),
		//	"orbitName": orbitName,
		//})

		//для лаб4 нужен жсон
		c.JSON(http.StatusOK, gin.H{
			"orbits":    a.repo.FilterOrbits(foundOrbits),
			"orbitName": orbitName,
		})
	}
}

func (a *Application) getDetailedOrbit(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	if orbit_name == "favicon.ico" {
		return
	}

	orbit, err := a.repo.GetOrbitByName(orbit_name)

	if err != nil {
		c.Error(err)
		return
	}

	c.HTML(http.StatusOK, "orbitDetail.html", gin.H{
		"Name":        orbit.Name,
		"Image":       orbit.Image,
		"Description": orbit.Description,
		"IsAvailable": orbit.IsAvailable,
		"Apogee":      orbit.Apogee,
		"Perigee":     orbit.Perigee,
		"Inclination": orbit.Inclination,
	})

}

func (a *Application) changeOrbitStatus(c *gin.Context) {
	orbitName := c.Param("orbit_name")

	// Call the modified ChangeAvailability method
	err := a.repo.ChangeOrbitStatus(orbitName)

	if err != nil {
		c.Error(err)
		return
	}

	c.Redirect(http.StatusFound, "/")
}

func (a *Application) newOrbit(c *gin.Context) {
	var requestBody ds.Orbits

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
	}

	err := a.repo.AddOrbit(requestBody.Name, requestBody.Apogee, requestBody.Perigee,
		requestBody.Inclination, requestBody.Description, requestBody.Image)
	log.Println(requestBody.Name, " is added")

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"Name":        requestBody.Name,
		"Apogee":      requestBody.Apogee,
		"Perigee":     requestBody.Perigee,
		"Inclination": requestBody.Inclination,
		"Description": requestBody.Description,
	})
}

func (a *Application) editOrbit(c *gin.Context) {
	orbit_name := c.Param("orbit_name")
	orbit, err := a.repo.GetOrbitByName(orbit_name)

	var editingOrbit ds.Orbits

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
	})
}

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

	request := &ds.TransferRequests{}
	request, err = a.repo.CreateTransferRequest(jsonMap["client_id"])
	if err != nil {
		c.Error(err)
		return
	}

	err = a.repo.AddTransferToOrbits(int(orbit.ID), int(request.ID))
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

	requests, err := a.repo.GetRequestByID(req_id)
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

func (a *Application) moderChangeTransferStatus(c *gin.Context) {
	var requestBody ds.ChangeTransferStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	user_isModer, err := a.repo.GetUserRole(requestBody.UserName)

	if err != nil {
		c.Error(err)
		return
	}

	if *user_isModer != true {
		c.String(http.StatusBadRequest, "У пользователя должна быть роль модератора")
		return
	}

	err = a.repo.ChangeRequestStatus(requestBody.TransferID, requestBody.Status)

	if err != nil {
		c.Error(err)
		return
	}

	c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
}

// надо ли делать проверку является ли пользователь клиентом?
func (a *Application) clientChangeTransferStatus(c *gin.Context) {
	var requestBody ds.ChangeTransferStatusRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	err := a.repo.ChangeRequestStatus(requestBody.TransferID, requestBody.Status)

	if err != nil {
		c.Error(err)
		return
	}

	c.String(http.StatusCreated, "Текущий статус: ", requestBody.Status)
}

// удаление записи (одной) из м-м по двум айди
func (a *Application) deleteTransferToOrbit(c *gin.Context) {
	var requestBody ds.TransfersToOrbit

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
		return
	}

	err1, err2 := a.repo.DeleteTransferToOrbit(requestBody.RequestRefer, requestBody.OrbitRefer)

	if err1 != nil || err2 != nil {
		c.Error(err1)
		c.Error(err2)
		c.String(http.StatusBadRequest, "Bad Request")
		return
	}

	c.String(http.StatusCreated, "Transfer-to-Orbit m-m was deleted")
}
