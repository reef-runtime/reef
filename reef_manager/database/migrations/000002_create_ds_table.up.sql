CREATE TABLE
IF NOT EXISTS
dataset(
    id   VARCHAR(64) NOT NULL,
    name TEXT NOT NULL,
    size INT NOT NULL, -- Size of the dataset in bytes.
    PRIMARY KEY (id)
);
