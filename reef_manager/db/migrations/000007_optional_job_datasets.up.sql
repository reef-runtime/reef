ALTER TABLE job
DROP COLUMN dataset_id;

CREATE TABLE
IF NOT EXISTS
job_has_dataset(
    job_id   VARCHAR(64) NOT NULL REFERENCES job(id),
    dataset_id VARCHAR(64) NOT NULL REFERENCES dataset(id),
    PRIMARY KEY (job_id, dataset_id)
);
