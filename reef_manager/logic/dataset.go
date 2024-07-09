package logic

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/reef-runtime/reef/reef_manager/database"
)

const dataSetFileEnding = ".bin"
const defaultFilePermissions = 0700

type DatasetManagerT struct {
	DatasetPath    string
	EmptyDatasetID *string
}

var DatasetManager DatasetManagerT

//
// Saves and loads datasets from the database and interacts with the filesystem to store the
// contents of the dataset.
//

func (m *DatasetManagerT) AddDataset(name string, data []byte) (id string, err error) {
	idBinary := sha256.Sum256(append([]byte(name), data...))
	id = hex.EncodeToString(idBinary[0:])

	path := filepath.Join(m.DatasetPath, fmt.Sprintf("%s%s", id, dataSetFileEnding))

	if err := os.WriteFile(path, data, defaultFilePermissions); err != nil {
		return "", err
	}

	alreadyExists, err := database.AddDataset(database.Dataset{
		Id:   id,
		Name: name,
		Size: uint32(len(data)),
	})
	if err != nil {
		return "", err
	}

	if alreadyExists {
		log.Debugf("Dataset `%s` already exists in database", id)
	}

	return id, nil
}

func (m *DatasetManagerT) DeleteDataset(id string) (found bool, err error) {
	if found, err := database.DeleteDataset(id); err != nil || !found {
		return found, err
	}
	if err := os.Remove(m.getDatasetPath(id)); err != nil {
		return found, err
	}
	return true, nil
}

func (m *DatasetManagerT) DoesDatasetExist(id string) (bool, error) {
	if _, err := os.Stat(m.getDatasetPath(id)); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		log.Errorf("Dataset is not readable: %s", err.Error())
		return false, err
	}

	return true, nil
}

func (m *DatasetManagerT) LoadDataset(id string) (data []byte, found bool, err error) {
	if data, err = os.ReadFile(m.getDatasetPath(id)); err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

func (m *DatasetManagerT) addEmptyDataset() error {
	id, err := m.AddDataset("Empty Dataset", []byte{})
	if err != nil {
		log.Errorf("Failed to add empty dataset: %s", err.Error())
		return err
	}
	log.Tracef("Added Empty dataset with ID `%s`", id)
	m.EmptyDatasetID = &id
	return nil
}

func (m *DatasetManagerT) getDatasetPath(id string) (dspath string) {
	return path.Join(m.DatasetPath, fmt.Sprintf("%s%s", id, dataSetFileEnding))
}

func newDatasetManager(datasetPath string) (DatasetManagerT, error) {
	if err := os.MkdirAll(datasetPath, defaultFilePermissions); err != nil {
		return DatasetManager, fmt.Errorf("could not create dataset dir: %s", err.Error())
	}

	m := DatasetManagerT{
		DatasetPath:    datasetPath,
		EmptyDatasetID: nil,
	}

	if err := m.addEmptyDataset(); err != nil {
		return DatasetManagerT{}, fmt.Errorf("add empty dataset: %s", err.Error())
	}

	return m, nil
}
