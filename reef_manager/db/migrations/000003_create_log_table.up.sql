CREATE TABLE
IF NOT EXISTS
log(
    -- Auto-incrementing ID of the log entry.
    id      SERIAL NOT NULL,

    -- Untyped in the database, can mean different things. (log level for instance)
    kind    SMALLINT NOT NULL,

    content TEXT NOT NULL,
    created TIMESTAMP WITH TIME ZONE NOT NULL,
    job_id  VARCHAR(64) NOT NULL REFERENCES job(id),
    PRIMARY KEY (id)
);
