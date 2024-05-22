package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/database"
)

func GetLogs(ctx *gin.Context) {
	amount, err := strconv.Atoi(ctx.Query("amount"))
	if err != nil {
		badRequest(ctx, "invalid amount value")
	}

	jobID := ctx.Query("jobid")

	logs, err := database.GetLastLogs(uint64(amount), jobID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, logs)
}
