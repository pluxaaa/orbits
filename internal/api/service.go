package api

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"strings"
)

type Orbit struct {
	ID    int
	Title string
	Desc  string
	Image string
	Short string
}

func StartServer() {

	log.Println("Server start up")

	Desc := []string{
		"Геостационарная орбита - круговая орбита, расположенная над экватором Земли (0° широты), находясь на которой, искусственный спутник обращается вокруг планеты с угловой скоростью, равной угловой скорости вращения Земли вокруг оси. В горизонтальной системе координат направление на спутник не изменяется ни по азимуту, ни по высоте над горизонтом — спутник «висит» в небе неподвижно. Поэтому спутниковая антенна, однажды направленная на такой спутник, всё время остаётся направленной на него. Геостационарная орбита является разновидностью геосинхронной орбиты и используется для размещения искусственных спутников (коммуникационных, телетрансляционных и т. п.).",
		"Средняя околоземная орбита — область космического пространства вокруг Земли, расположенная над низкой околоземной орбитой (от 160 до 2000 км над уровнем моря) и под геосинхронной орбитой (35 786 км над уровнем моря). На средней околоземной орбите находятся искусственные спутники, используемые в основном для навигации, связи, исследования Земли и космоса. Наиболее распространенная высота составляет примерно 20 200 километров при орбитальном периоде 12 часов. На данной высоте расположены спутники GPS. Также на средней околоземной орбите расположены спутники ГЛОНАСС (высотой 19 100 километров), Галилео (высота 23 222 километра), O3b (высота 8 063 км). Орбитальные периоды спутников на средней околоземной орбите колеблются от 2 до 24 часов.",
		"Космическая орбита вокруг Земли, имеющая высоту над поверхностью планеты в диапазоне от 160 км (период обращения около 88 минут) до 2000 км (период около 127 минут). Объекты, находящиеся на высотах менее 160 км, испытывают очень сильное влияние атмосферы и нестабильны. За исключением пилотируемых полётов к Луне (программа Аполлон, США), все космические полеты человека проходили либо в области НОО, либо являлись суборбитальными. Наибольшую высоту среди пилотируемых полётов в области НОО имел аппарат Gemini 11, с апогеем в 1374 км. На настоящий момент все обитаемые космические станции и большая часть искусственных спутников Земли используют или использовали НОО. Также на НОО сосредоточена большая часть космического мусора.",
	}

	Orbits := []Orbit{
		{0, "Геостационарная орбита", Desc[0], "../resources/GEO.png", "GEO"},
		{1, "Средняя околоземная орбита", Desc[1], "../resources/MEO.png", "MEO"},
		{2, "Низкая околоземная орбита", Desc[2], "../resources/LEO.png", "LEO"},
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
					"Title": Orbits[i].Title,
					"Image": Orbits[i].Image,
					"Desc":  Orbits[i].Desc,
				})
			}
		}
	})

	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")

	log.Println("Server down")
}
