package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/reef-runtime/reef/reef_manager/logic"
)

func GetJobs(c *fiber.Ctx) error {
	jobs, err := database.ListJobs()
	if err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(jobs)
}

//
// Job submission.
//

type JobIDResponse struct {
	ID string `json:"id"`
}

func SubmitJob(c *fiber.Ctx) error {
	var submission logic.JobSubmission

	if err := c.BodyParser(&submission); err != nil {
		return err
	}

	id, err := logic.SubmitJob(submission)
	if err != nil {
		return fiber.ErrInternalServerError
	}

	return c.JSON(JobIDResponse{
		ID: id,
	})
}
