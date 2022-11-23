package store

import "github.com/gogama/reee-evolution/daemon"

type TempStore struct {
}

func (s *TempStore) GetMetadata(storeID string) (daemon.Metadata, bool) {
	return daemon.Metadata{}, false
}

func (s *TempStore) PutMessage(storeID string, msg *daemon.Message) error {
	return nil
}

func (s *TempStore) RecordEval(storeID string, r daemon.EvalRecord) error {
	return nil
}
