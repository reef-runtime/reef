CREATE TABLE
IF NOT EXISTS
job(
    id   BYTEA,
    name TEXT,
    submitted TIMESTAMP WITH TIME ZONE,
    status SMALLINT,
    PRIMARY KEY (id)
);
