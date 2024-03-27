package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/prasher1421/go-jwt/controllers"
	"github.com/prasher1421/go-jwt/middleware"
)

func UserRoutes(incomingRoutes *gin.Engine){
	incomingRoutes.Use(middleware.Authenticate())
	incomingRoutes.GET("/users",controllers.GetUsers())
	incomingRoutes.GET("/users/:user_id",controllers.GetUser())
}