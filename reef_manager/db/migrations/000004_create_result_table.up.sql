CREATE TABLE
IF NOT EXISTS
job_result(
    -- Foreign key: job ID.
    job_id          VARCHAR(64) NOT NULL REFERENCES job(id),
    -- Whether the job completed successfully or failed.
    success          BOOLEAN NOT NULL,
    -- Untyped byte slice in the database, can mean different things.
    content         BYTEA NOT NULL,
    -- How the frontend should interpret the content (display as string or hex for instance).
    content_type    SMALLINT NOT NULL,
    -- Useful for determining job runtime.
    created         TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (job_id)
);
