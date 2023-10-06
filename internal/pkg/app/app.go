package app

import (
	"L1/internal/app/ds"
	"L1/internal/app/dsn"
	"L1/internal/app/repository"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
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

	a.r.GET("/", a.loadGeneral)
	a.r.GET("/:orbit_name", a.loadDetail)
	a.r.GET("/newOrbit", a.newOrbit)
	a.r.GET("/editOrbit", a.editOrbit)
	a.r.POST("/chStatusOrbit/:orbit_name", a.changeOrbitStatus)
	a.r.GET("/:orbit_name/add", a.addOrbitToRequest)

	a.r.Run(":8000")

	log.Println("Server is down")
}

func (a *Application) loadGeneral(c *gin.Context) {
	orbitName := c.Query("orbit_name")

	if orbitName == "" {
		log.Println("ALL ORBITS 1")

		allOrbits, err := a.repo.GetAllOrbits()

		if err != nil {
			c.Error(err)
		}

		c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
			"orbits": a.repo.FilterOrbits(allOrbits),
		})
	} else {
		log.Println("!!! SEARCHING ORBITS !!!")

		foundOrbits, err := a.repo.SearchOrbits(orbitName)
		if err != nil {
			c.Error(err)
			return
		}

		c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
			"orbits":    a.repo.FilterOrbits(foundOrbits),
			"orbitName": orbitName,
		})
	}
}

func (a *Application) loadDetail(c *gin.Context) {
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
	var requestBody ds.AddOrbitRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
	}

	err := a.repo.AddOrbit(requestBody.Name, requestBody.Apogee, requestBody.Perigee,
		requestBody.Inclination, requestBody.Description)
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
	var requestBody ds.EditOrbitNameRequestBody

	if err := c.BindJSON(&requestBody); err != nil {
		c.Error(err)
	}

	err := a.repo.EditOrbitName(requestBody.OldName, requestBody.NewName)

	if err != nil {
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"old_name": requestBody.OldName,
		"new_name": requestBody.NewName,
	})
}

func (a *Application) addOrbitToRequest(c *gin.Context) {
	orbit_name := c.Param("orbit_name")

	//получение инфы об орбите (id)
	orbit, err := a.repo.GetOrbitByName(orbit_name)

	if err != nil {
		c.Error(err)
		return
	}

	request := &ds.TransferRequests{}
	request, err = a.repo.CreateTransferRequest(1)
	if err != nil {
		c.Error(err)
		return
	}
	log.Println("REQUEST ID: ", request.ID, "\nORBIT ID: ", orbit.ID)

	err = a.repo.AddTransferToOrbits(int(orbit.ID), int(request.ID))
	if err != nil {
		c.Error(err)
		return
	}

}
