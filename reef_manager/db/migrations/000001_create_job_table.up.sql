CREATE TABLE
IF NOT EXISTS
job(
    id   VARCHAR(64),
    name TEXT,
    submitted TIMESTAMP WITH TIME ZONE,
    status SMALLINT,
    PRIMARY KEY (id)
);
