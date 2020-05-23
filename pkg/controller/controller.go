package controller

import "github.com/gin-gonic/gin"

type Controller interface {
	Execute(g *gin.Context)
}
