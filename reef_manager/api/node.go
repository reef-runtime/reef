package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

func GetNodes(ctx *gin.Context) {
	nodes := logic.NodeManager.ListNodes()
	ctx.JSON(http.StatusOK, nodes)
}
