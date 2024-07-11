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
	// Attaching a dataset to a job submission is non-optional.
	DatasetID  string                       `json:"datasetId"`
	SourceCode string                       `json:"sourceCode"`
	Language   logic.JobProgrammingLanguage `json:"language"`
}

type SingleJobResponse struct {
	Job  logic.APIJob
	Logs []database.JobLog
}

func GetJobs(ctx *gin.Context) {
	jobs, err := logic.JobManager.ListJobs()
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	ctx.JSON(http.StatusOK, jobs)
}

// Returns a job, including logs.
// NOTE: this is special for this get-job function.
func GetJob(ctx *gin.Context) {
	jobID := ctx.Param(jobIDUrlParam)

	job, found, err := logic.JobManager.GetJob(jobID, true)
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

	ctx.JSON(http.StatusOK, job)
}

func GetTemplates(ctx *gin.Context) {
	templates := logic.JobManager.ListTemplates()
	ctx.JSON(http.StatusOK, templates)
}

//
// Job submission.
//

func SubmitJob(ctx *gin.Context) {
	session := extractSession(ctx)
	var submission JobSubmission

	if err := ctx.ShouldBindJSON(&submission); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	//
	// Validate additional constraints, like validity of the dataset and language.
	//

	found, err := logic.DatasetManager.DoesDatasetExist(submission.DatasetID)
	if err != nil {
		serverErr(ctx, err.Error())
		return
	}

	if !found {
		badRequest(ctx, fmt.Sprintf("dataset with id `%s` not found", submission.DatasetID))
		return
	}

	if err := submission.Language.Validate(); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	//
	// Submit job internally.
	//

	id, compileErr, systemErr := logic.JobManager.SubmitJob(
		submission.Language,
		submission.SourceCode,
		submission.Name,
		submission.DatasetID,
		session.Id,
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
		IdBody{
			Id: id,
		},
	)
}

//
// Job cancellation and abortion.
//

func userHasJobAccess(jobID string, user AuthResponse) (bool, error) {
	if user.IsAdmin {
		return true, nil
	}

	hasAccess, err := database.JobHasOwner(jobID, user.Id)
	if err != nil {
		return false, err
	}

	return hasAccess, nil
}

func AbortOrCancelJob(ctx *gin.Context) {
	session := extractSession(ctx)
	var job IdBody

	if err := ctx.ShouldBindJSON(&job); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	hasAccess, err := userHasJobAccess(job.Id, session)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if !hasAccess {
		respondErr(ctx, "could not abort job", "forbidden", http.StatusForbidden)
		return
	}

	found, err := logic.JobManager.AbortJob(job.Id)
	if err != nil {
		ctx.Status(http.StatusInternalServerError)
		return
	}

	if !found {
		respond(
			ctx,
			newErrResponse("could not abort/cancel job", "job does not exist or is already in the <done> state"),
			http.StatusUnprocessableEntity,
		)
		return
	}

	respondOk(ctx, "aborted job")
}

func GetResult(ctx *gin.Context) {
	var id IdBody

	if err := ctx.ShouldBindJSON(&id); err != nil {
		badRequest(ctx, err.Error())
		return
	}

	result, found, err := database.GetResult(id.Id)
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
