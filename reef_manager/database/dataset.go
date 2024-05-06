package database

const DSTableName = "dataset"

type Dataset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size uint32 `json:"size"` // Size of the dataset in bytes.
}

func AddDataset(dataset Dataset) error {
	if _, err := db.builder.Insert(JobTableName).Values(
		dataset.ID,
		dataset.Name,
		dataset.Size,
	).Exec(); err != nil {
		log.Errorf("Could not add dataset to database: executing query failed: %s", err.Error())
		return err
	}

	return nil
}

func DeleteDataset(datasetID string) (found bool, err error) {
	res, err := db.builder.Delete(DSTableName).Where("dataset.ID=?", datasetID).Exec()
	if err != nil {
		return false, err
	}

	affected, err := res.RowsAffected()
	if err != nil {
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
			ds.ID,
			ds.Name,
			ds.Size,
		); err != nil {
			log.Errorf("Could not list datasets: scanning results failed: %s", err.Error())
			return nil, err
		}

		datasets = append(datasets, ds)
	}

	return datasets, nil
}
