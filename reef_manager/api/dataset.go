package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

type DatasetIDBody struct {
	ID string `json:"id"`
}

func GetDatasets(ctx *gin.Context) {
	datasets, err := database.ListDatasets()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, datasets)
}

func UploadDataset(ctx *gin.Context) {
	fileHeader, err := ctx.FormFile("dataset")
	if err != nil {
		ctx.Status(http.StatusBadRequest)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
	}
	id, err := logic.AddDataset(fileHeader.Filename, data)
	if err != nil {
		respond(
			ctx,
			newErrResponse("could not upload dataset", err.Error()),
			http.StatusUnprocessableEntity,
		)
	}
	ctx.JSON(
		http.StatusOK,
		DatasetIDBody{
			ID: id,
		},
	)
}

func DeleteDataset(ctx *gin.Context) {
	var dataset DatasetIDBody

	if err := ctx.ShouldBindJSON(&dataset); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	found, err := logic.DeleteDataset(dataset.ID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if !found {
		respond(
			ctx,
			newErrResponse("could not delete dataset", "dataset does not exist"),
			http.StatusUnprocessableEntity,
		)
		return
	}

	respondOk(ctx, "deleted dataset")
}
