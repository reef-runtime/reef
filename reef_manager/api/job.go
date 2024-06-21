package api

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

const jobIDUrlParam = "job_id"

type JobSubmission struct {
	Name string `json:"name"`
	// Attaching a dataset to a job submission is optimal.
	DatasetID  *string                      `json:"datasetId"`
	SourceCode string                       `json:"sourceCode"`
	Language   logic.JobProgrammingLanguage `json:"language"`
}

type JobResponse struct {
	Name     string            `json:"name"`
	Logs     []database.JobLog `json:"logs"`
	State    []byte            `json:"state"`
	Progress float32           `json:"progress"`
	Result   *database.Result  `json:"result"`
}

func GetJobs(ctx *gin.Context) {
	jobs, err := logic.JobManager.ListJobs()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, jobs)
}

func GetJob(ctx *gin.Context) {
	jobID := ctx.Param(jobIDUrlParam)

	job, found, err := database.GetJob(jobID)
	if err != nil {
		serverErr(ctx, err.Error())
		return
	}

	if !found {
		respondErr(
			ctx,
			"illegal job",
			fmt.Sprintf("job with id `%s` not found", jobID),
			http.StatusUnprocessableEntity,
		)
		return
	}

	logs, err := database.GetLastLogs(nil, jobID)
	if err != nil {
		serverErr(ctx, err.Error())
		return
	}

	result, resultFound, err := database.GetResult(jobID)
	if err != nil {
		serverErr(ctx, err.Error())
		return
	}

	jobResponse := JobResponse{
		Name:     job.Name,
		Logs:     logs,
		State:    nil, // TODO: get state.
		Progress: 0,   // TODO: get progress.
		Result:   nil, // TODO: get result.
	}

	// Attach result if it exists.
	if resultFound {
		jobResponse.Result = &result
	}

	ctx.JSON(http.StatusOK, jobResponse)
}

//
// Job submission.
//

func SubmitJob(ctx *gin.Context) {
	var submission JobSubmission

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
	id, compileErr, systemErr := logic.JobManager.SubmitJob(
		submission.Language,
		submission.SourceCode,
		submission.Name,
	)

	if systemErr != nil {
		serverErr(ctx, systemErr.Error())
		return
	}

	// Notify the user about a potential compilation error.
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
