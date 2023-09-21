package app

import (
	"log"
	"net/http"

	"L1/internal/app/dsn"
	"L1/internal/app/repository"

	"github.com/gin-gonic/gin"
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

	a.r.GET("/", a.loadHome)
	a.r.GET("/:orbit_name", a.loadPage)

	a.r.Run(":8000")

	log.Println("Server is down")
}

func (a *Application) loadHome(c *gin.Context) {
	orbit_name := c.Query("orbit_name")

	if orbit_name == "" {
		log.Println("ALL ORBITS 1")

		all_orbits, err := a.repo.GetAllOrbits()

		if err != nil {
			c.Error(err)
		}

		c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
			"orbits": a.repo.FilterOrbits(all_orbits),
		})
	} else {
		found_orbits, err := a.repo.SearchOrbits(orbit_name)

		if err != nil {
			c.Error(err)
			return
		}

		c.HTML(http.StatusOK, "regions.html", gin.H{
			"regions": a.repo.FilterOrbits(found_orbits),
		})
	}
}

func (a *Application) loadPage(c *gin.Context) {
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
	})

}
