package store

import (
	"errors"

	"github.com/miekg/dns"
	"github.com/syndtr/goleveldb/leveldb"
)

type Store interface {
	Get(key string) (dns.RR, error)
	Delete(key string) error
	Set(key string, answer dns.RR) error
	List() (ret []dns.RR, err error)
	Close() error
}

type leveldbStore struct {
	db *leveldb.DB
}

func NewLeveldbStore() (Store, error) {
	db, err := leveldb.OpenFile("zone.db", nil)
	if err != nil {
		return nil, err
	}

	return &leveldbStore{
		db: db,
	}, nil
}

// 如果没有，则返回
func (s *leveldbStore) Get(fqdn string) (dns.RR, error) {
	value, err := s.db.Get([]byte(fqdn), nil)
	if err != nil {
		if errors.Is(err, leveldb.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return dns.NewRR(string(value))

}

func (s *leveldbStore) Delete(fqdn string) error {
	return s.db.Delete([]byte(fqdn), nil)
}

// store rr string
func (s *leveldbStore) Set(fqdn string, rr dns.RR) error {
	return s.db.Put([]byte(fqdn), []byte(rr.String()), nil)
}

func (s *leveldbStore) List() (ret []dns.RR, err error) {

	iter := s.db.NewIterator(nil, nil)
	for iter.Next() {
		value := iter.Value()
		rr, err := dns.NewRR(string(value))
		if err != nil {
			return nil, err
		}
		ret = append(ret, rr)
	}
	// 释放迭代器
	iter.Release()
	err = iter.Error()
	return ret, err
}

func (s *leveldbStore) Close() error {
	return s.db.Close()
}
