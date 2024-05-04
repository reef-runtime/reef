package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gofiber/fiber/v2"
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

type JobIDBody struct {
	ID string `json:"id"`
}

func SubmitJob(ctx *gin.Context) {
	var submission logic.JobSubmission

	if err := ctx.ShouldBindJSON(&submission); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	id, err := logic.JobManager.SubmitJob(submission)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(
		fiber.StatusOK,
		JobIDBody{
			ID: id,
		},
	)
}

//
// Job cancellation and abortion.
//

func AbortJob(ctx *gin.Context) {
	var job JobIDBody

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
