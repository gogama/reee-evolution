package store

import "github.com/gogama/reee-evolution/daemon"

type NullStore struct {
}

func (s *NullStore) GetMetadata(_ string) (daemon.Metadata, bool, error) {
	return daemon.Metadata{}, false, nil
}

func (s *NullStore) PutMessage(_ string, _ *daemon.Message) error {
	return nil
}

func (s *NullStore) RecordEval(_ string, _ *daemon.EvalRecord) error {
	return nil
}
