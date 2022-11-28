package store

import "github.com/gogama/reee-evolution/daemon"

type NullStore struct {
}

func (s *NullStore) GetMetadata(storeID string) (daemon.Metadata, bool, error) {
	return daemon.Metadata{}, false, nil
}

func (s *NullStore) PutMessage(storeID string, msg *daemon.Message) error {
	return nil
}

func (s *NullStore) RecordEval(storeID string, r daemon.EvalRecord) error {
	return nil
}
