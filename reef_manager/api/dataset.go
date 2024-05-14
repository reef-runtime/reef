package api

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

const formFileFieldName = "dataset"

type IDBody struct {
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
	fileHeader, err := ctx.FormFile(formFileFieldName)
	if err != nil {
		badRequest(
			ctx,
			err.Error(),
		)
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	id, err := logic.DatasetManager.AddDataset(fileHeader.Filename, data)
	if err != nil {
		respond(
			ctx,
			newErrResponse("could not upload dataset", err.Error()),
			http.StatusUnprocessableEntity,
		)
		return
	}

	ctx.JSON(
		http.StatusOK,
		IDBody{
			ID: id,
		},
	)
}

func DeleteDataset(ctx *gin.Context) {
	var dataset IDBody

	if err := ctx.ShouldBindJSON(&dataset); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	found, err := logic.DatasetManager.DeleteDataset(dataset.ID)
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
