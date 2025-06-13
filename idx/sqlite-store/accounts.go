package sqlitestore

import (
	"context"
	"log"

	"github.com/chitransh-rockwallet/1sat-indexer/idx"
)

func (s *SQLiteStore) AcctsByOwners(ctx context.Context, owners []string) ([]string, error) {
	if len(owners) == 0 {
		return nil, nil
	}
	query := `SELECT account FROM owner_accounts WHERE owner IN (` + placeholders(len(owners)) + `)`
	rows, err := s.READDB.QueryContext(ctx, query, toInterfaceSlice(owners)...)
	if err != nil {
		log.Panic(err)
		return nil, err
	}
	defer rows.Close()
	accts := make([]string, 0, 4)
	for rows.Next() {
		var acct string
		if err = rows.Scan(&acct); err != nil {
			log.Panic(err)
			return nil, err
		}
		accts = append(accts, acct)
	}
	return accts, nil
}

func (s *SQLiteStore) AcctOwners(ctx context.Context, account string) ([]string, error) {
	rows, err := s.READDB.QueryContext(ctx, `SELECT owner FROM owner_accounts WHERE account = ?`, account)
	if err != nil {
		log.Panic(err)
		return nil, err
	}
	defer rows.Close()
	owners := make([]string, 0, 4)
	for rows.Next() {
		var owner string
		if err = rows.Scan(&owner); err != nil {
			log.Panic(err)
			return nil, err
		}
		owners = append(owners, owner)
	}
	return owners, nil
}

func (s *SQLiteStore) UpdateAccount(ctx context.Context, account string, owners []string) error {
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		if _, err := insOwnerAcct.ExecContext(ctx, owner, account, account); err != nil {
			log.Panic(err)
			return err
		} else if _, err := s.LogOnce(ctx, idx.OwnerSyncKey, owner, 0); err != nil {
			log.Panic(err)
			return err
		}
	}
	return nil
}
