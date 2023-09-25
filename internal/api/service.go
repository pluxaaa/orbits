package api

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

type Orbit struct {
	ID          int
	Title       string
	Desc        string
	Apogee      string
	Perigee     string
	Inclination string
	Image       string
	Short       string
}

func StartServer() {
	//test commit lab1 branch

	log.Println("Server start up")

	Props := [][]string{
		{"35786 км", "35786 км", "0°"}, {"700 км", "700 км", "90°"}, {"39000 км", "700 км", "63,4°"},
	}

	Desc := []string{
		"Геостационарная орбита - круговая орбита, расположенная над экватором Земли, находясь на которой, искусственный спутник обращается вокруг планеты с угловой скоростью, равной угловой скорости вращения Земли вокруг оси. В горизонтальной системе координат направление на спутник не изменяется ни по азимуту, ни по высоте над горизонтом — спутник «висит» в небе неподвижно. Поэтому спутниковая антенна, однажды направленная на такой спутник, всё время остаётся направленной на него. Геостационарная орбита является разновидностью геосинхронной орбиты и используется для размещения искусственных спутников (коммуникационных, телетрансляционных и т. п.).",
		"Орбита космического аппарата (спутника), имеющая наклонение к плоскости экватора в 90°. Полярные орбиты относятся к Кеплеровским орбитам. Трасса орбиты полярного спутника проходит над всеми широтами Земли, в отличие от спутников с наклонением орбиты меньше 90°.",
		"Орбита «Молния» — один из типов высокой эллиптической орбиты с наклонением в 63,4°, аргументом перицентра −90° и периодом обращения в половину звёздных суток. Данный тип орбиты получил название по серии советских космических аппаратов «Молния» двойного назначения, впервые использовавших эту орбиту в своей работе.",
	}

	Orbits := []Orbit{
		{0, "Геостационарная орбита", Desc[0], Props[0][0], Props[0][1], Props[0][2],
			"../resources/GEO.png", "GEO"},
		{1, "Полярная орибат", Desc[1], Props[1][0], Props[1][1], Props[1][2],
			"../resources/MEO.png", "MEO"},
		{2, "Орбита 'Молния'", Desc[2], Props[2][0], Props[2][1], Props[2][2],
			"../resources/Molniya.png", "LEO"},
	}

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	/*загружает все файлы из templates
	//глоб-шаблоны исп. для поиска файлов
	//* - "всё" */

	r.Static("/resources", "./resources") //загрузка статических данных
	r.Static("/css", "./templates")

	/*не требуют динамической обработки на сервере (css, js ...)
	//relPath - URL-prefix

	/*Context is the most important part of gin. It allows us to pass variables between middleware,
	manage the flow, validate the JSON of a request and render a JSON response for example.

	func(c *gin.Context) - функция-обработчик, которая будет вызвана при получении
	HTTP GET запроса на путь "/home";
	внутри этой функции происходит обработка запроса и формирование ответа.*/

	r.GET("/home", func(c *gin.Context) {

		query := c.DefaultQuery("query", "")
		//если что-то введено, то query="что ввели"
		//иначе пустой: query=""

		//поиск в Orbits основываясь на query
		//здесь сначала получаем объекты для отображения - либо фильтр, либо все
		var filteredOrbits []Orbit
		if query != "" {
			for i := 0; i < len(Orbits); i++ {
				if strings.Contains(strings.ToLower(Orbits[i].Title), strings.ToLower(query)) {
					filteredOrbits = append(filteredOrbits, Orbits[i])
				}
			}
		} else {
			filteredOrbits = Orbits // если запрос пустой, то все орбиты
			log.Println("nob")
		}

		//выводим отфильтрованные (/все)
		c.HTML(http.StatusOK, "orbitsGeneral.html", gin.H{
			"orbits": filteredOrbits,
			"query":  query, //чтобы отображалось в html в {{ .query }}
		})
	})

	r.GET("/home/:Short", func(c *gin.Context) {
		short := c.Param("Short")

		for i := range Orbits {
			if short == Orbits[i].Short {
				c.HTML(http.StatusOK, "orbitDetail.html", gin.H{
					"Title":       Orbits[i].Title,
					"Image":       Orbits[i].Image,
					"Desc":        Orbits[i].Desc,
					"Apogee":      Orbits[i].Apogee,
					"Perigee":     Orbits[i].Perigee,
					"Inclination": Orbits[i].Inclination,
				})
			}
		}
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	log.Println("Server down")

}
