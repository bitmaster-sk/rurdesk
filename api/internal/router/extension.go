package router

import "github.com/gin-gonic/gin"

type RouteGroups struct {
	Engine  *gin.Engine
	Public  *gin.RouterGroup
	Private *gin.RouterGroup
	Admin   *gin.RouterGroup
}

type Extension func(RouteGroups)

func NoExtension(RouteGroups) {}
