CREATE TABLE
IF NOT EXISTS
job(
    id   VARCHAR(64) NOT NULL,
    name TEXT NOT NULL,
    submitted TIMESTAMP WITH TIME ZONE NOT NULL,
    status SMALLINT NOT NULL,
    PRIMARY KEY (id)
);

--
-- | id | name | submitted | status |
-- |    |      |           |        |