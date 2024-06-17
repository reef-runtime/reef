package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

func Init(logger *logrus.Logger) {
	log = logger
	log.Debug("API successfully initialized")
}

type responseT struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func newOkResponse(message string) responseT {
	return responseT{
		Success: true,
		Message: message,
		Error:   "",
	}
}

func newErrResponse(message, err string) responseT {
	return responseT{
		Success: false,
		Message: message,
		Error:   err,
	}
}

func respond(ctx *gin.Context, res responseT, code int) {
	ctx.JSON(code, res)
}

func respondOk(ctx *gin.Context, message string) {
	respond(ctx, newOkResponse(message), http.StatusOK)
}

func respondErr(ctx *gin.Context, message string, err string, code int) {
	respond(ctx, newErrResponse(message, err), code)
}

func badRequest(ctx *gin.Context, err string) {
	respond(ctx, newErrResponse("bad request", err), http.StatusBadRequest)
}

func serverErr(ctx *gin.Context, err string) {
	log.Errorf("Internal server error: %s", err)
	respond(ctx, newErrResponse("internal server error", "backend error"), http.StatusInternalServerError)
}
