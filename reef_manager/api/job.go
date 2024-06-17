package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

func GetJobs(ctx *gin.Context) {
	jobs, err := database.ListJobs()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, jobs)
}

//
// Job submission.
//

func SubmitJob(ctx *gin.Context) {
	var submission logic.JobSubmission

	if err := ctx.ShouldBindJSON(&submission); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	// Validate additional constraints, like validity of the dataset and language.
	if submission.DatasetID != nil {
		found, err := logic.DatasetManager.DoesDatasetExist(*submission.DatasetID)
		if err != nil {
			serverErr(ctx, err.Error())
			return
		}

		if !found {
			badRequest(ctx, fmt.Sprintf("dataset with id `%s` not found", *submission.DatasetID))
			return
		}
	}

	if err := submission.Language.Validate(); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	// Submit job internally.
	id, compileErr, systemErr := logic.JobManager.SubmitJob(submission)
	if systemErr != nil {
		serverErr(ctx, systemErr.Error())
		return
	}

	// Notify user about potential compile error.
	if compileErr != nil {
		respondErr(ctx, "compilation error", *compileErr, http.StatusUnprocessableEntity)
		return
	}

	ctx.JSON(
		http.StatusOK,
		IDBody{
			ID: id,
		},
	)
}

//
// Job cancellation and abortion.
//

func AbortJob(ctx *gin.Context) {
	var job IDBody

	if err := ctx.ShouldBindJSON(&job); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	found, err := logic.JobManager.AbortJob(job.ID)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if !found {
		respond(
			ctx,
			newErrResponse("could not abort job", "job does not exist or is not in <queued> state"),
			http.StatusUnprocessableEntity,
		)
		return
	}

	respondOk(ctx, "aborted job")
}

func GetResult(ctx *gin.Context) {
	var id IDBody

	if err := ctx.ShouldBindJSON(&id); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	result, found, err := database.GetResult(id.ID)
	if !found {
		respond(
			ctx,
			newErrResponse("could not get result", "result doesnt exist yet"),
			http.StatusUnprocessableEntity,
		)
		return
	}

	if err != nil {
		ctx.Status(http.StatusInternalServerError)
	}

	ctx.JSON(
		http.StatusOK,
		result,
	)
}
