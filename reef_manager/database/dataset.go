package database

import (
	"database/sql"
	"errors"
)

const DSTableName = "dataset"

type Dataset struct {
	// Is guaranteed to be 64 chars long.
	Id   string `json:"id"`
	Name string `json:"name"`
	Size uint32 `json:"size"` // Size of the dataset in bytes.
}

func AddDataset(dataset Dataset) (alreadyExists bool, err error) {
	var _id string
	err = db.builder.Select("id").From(DSTableName).Where("id=?", dataset.Id).QueryRow().Scan(&_id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}

	// Dataset already exists in database.
	if err == nil {
		return true, nil
	}

	if _, err := db.builder.Insert(DSTableName).Values(
		dataset.Id,
		dataset.Name,
		dataset.Size,
	).Exec(); err != nil {
		log.Errorf("Could not add dataset to database: executing query failed: %s", err.Error())
		return false, err
	}

	return false, nil
}

func DeleteDataset(datasetId string) (found bool, err error) {
	res, err := db.builder.Delete(DSTableName).Where("dataset.Id=?", datasetId).Exec()
	if err != nil {
		log.Errorf("Could not delete database: executing query failed: %s", err.Error())
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
		log.Errorf("Could not delete database: getting rows affected failed: %s", err.Error())
		return false, err
	}

	return affected != 0, nil
}

func ListDatasets() ([]Dataset, error) {
	baseQuery := db.builder.Select("*").From(DSTableName).OrderBy("name ASC")

	res, err := baseQuery.Query()
	if err != nil {
		log.Errorf("Could not list datasets: executing query failed: %s", err.Error())
		return nil, err
	}

	datasets := make([]Dataset, 0)

	for res.Next() {
		var ds Dataset
		if err := res.Scan(
			&ds.Id,
			&ds.Name,
			&ds.Size,
		); err != nil {
			log.Errorf("Could not list datasets: scanning results failed: %s", err.Error())
			return nil, err
		}

		datasets = append(datasets, ds)
	}
	return datasets, nil
}
