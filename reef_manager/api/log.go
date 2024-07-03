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

	jobId := ctx.Query("jobid")

	amountUint64 := uint64(amount)

	logs, err := database.GetLastLogs(&amountUint64, jobId)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, logs)
}
