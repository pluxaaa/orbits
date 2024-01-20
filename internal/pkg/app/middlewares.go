package app

import (
	"L1/internal/app/ds"
	"L1/internal/app/role"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"log"
	"net/http"
	"strings"
)

const jwtPrefix = "Bearer "

func GetJWTToken(gCtx *gin.Context) (string, error) {
	jwtStr := gCtx.GetHeader("Authorization")

	if jwtStr == "" {
		log.Println("\ngetting JWT from cookie")
		var cookieErr error
		jwtStr, cookieErr = gCtx.Cookie("orbits-api-token")
		if cookieErr != nil {
			gCtx.AbortWithStatus(http.StatusBadRequest)
			return "", cookieErr
		}
	}
	return jwtStr, nil
}

func GetUserClaims(jwtStr string, gCtx *gin.Context, a *Application) (*ds.JWTClaims, error) {
	token, err := jwt.ParseWithClaims(jwtStr, &ds.JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWT.Token), nil
	})
	if err != nil {
		gCtx.AbortWithStatus(http.StatusForbidden)
		log.Println(err)

		return nil, err
	}

	return token.Claims.(*ds.JWTClaims), nil
}

func (a *Application) WithAuthCheck(assignedRoles ...role.Role) func(context *gin.Context) {
	return func(c *gin.Context) {
		jwtStr, err := GetJWTToken(c)
		if err != nil {
			panic(err)
		}

		if jwtStr == "Bearer guest" {
			c.Set("userRole", 0)
			c.Set("userUUID", uuid.Nil)

			return
		}

		if !strings.HasPrefix(jwtStr, jwtPrefix) {
			c.AbortWithStatus(http.StatusForbidden)

			return
		}

		jwtStr = jwtStr[len(jwtPrefix):]

		err = a.redis.CheckJWTInBlackList(c.Request.Context(), jwtStr)
		if err == nil { // значит что токен в блеклисте
			c.AbortWithStatus(http.StatusForbidden)

			return
		}

		if !errors.Is(err, redis.Nil) { // значит что это не ошибка отсуствия - внутренняя ошибка
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		myClaims, err := GetUserClaims(jwtStr, c, a)
		if err != nil {
			log.Println("error GetUserClaims")
			return
		}

		isAssigned := false

		for _, oneOfAssignedRole := range assignedRoles {
			if myClaims.Role == oneOfAssignedRole {
				isAssigned = true
				break
			}
		}

		if !isAssigned {
			c.AbortWithStatus(http.StatusForbidden)
			log.Printf("Роль %d не указана в %d", myClaims.Role, assignedRoles)
			return
		}

		c.Set("userRole", myClaims.Role)
		c.Set("userUUID", myClaims.UserUUID)
	}

}
