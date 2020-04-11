package storage

import "github.com/syndtr/goleveldb/leveldb"

type levelInstance struct {
	db *leveldb.DB
}

func (i *levelInstance) Get(key []byte) ([]byte, error) {
	v, err := i.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil
	}
	return v, err
}
func (i *levelInstance) GetAll() (map[string][]byte, error) {
	result := map[string][]byte{}
	var err error
	iter := i.db.NewIterator(nil, nil)
	for iter.Next() {
		result[string(iter.Key())] = iter.Value()
	}
	iter.Release()
	err = iter.Error()
	return result, err
}
func (i *levelInstance) Set(key []byte, value []byte) error {
	return i.db.Put(key, value, nil)
}
func (i *levelInstance) Delete(key []byte) error {
	return i.db.Delete(key, nil)
}
func (i *levelInstance) Close() error {
	return i.db.Close()
}

func newLevelInstance(cfg *Config) (*levelInstance, error) {
	if cfg.Path == "" {
		cfg.Path = "db"
	}
	level, err := leveldb.OpenFile(cfg.Path, nil)
	return &levelInstance{level}, err
}
