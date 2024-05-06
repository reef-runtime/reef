package logic

//
// Saves and loads datasets from the database and interacts with the filesystem to store the
// contents of the dataset.
//

func AddDataset(name string, data []byte) (id string, err error) {
	panic("NOT IMPLEMENTED")
}

func DeleteDataset(id string) (found bool, err error) {
	panic("NOT IMPLEMENTED")
}

func LoadDataset(id string) (data []byte, found bool, err error) {
	panic("NOT IMPLEMENTED")
}
