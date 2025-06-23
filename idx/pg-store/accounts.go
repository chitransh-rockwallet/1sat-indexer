package pgstore

import (
	"context"
	"log"

	"github.com/chitransh-rockwallet/1sat-indexer/v5/idx"
)

func (p *PGStore) AcctsByOwners(ctx context.Context, owners []string) ([]string, error) {
	if len(owners) == 0 {
		return nil, nil
	}
	rows, err := p.DB.Query(ctx, `SELECT account 
		FROM owner_accounts 
		WHERE owner = ANY($1)`,
		owners,
	)
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

func (p *PGStore) AcctOwners(ctx context.Context, account string) ([]string, error) {
	if rows, err := p.DB.Query(ctx, `SELECT owner 
		FROM owner_accounts 
		WHERE account = $1`,
		account,
	); err != nil {
		return nil, err
	} else {
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
}

func (p *PGStore) UpdateAccount(ctx context.Context, account string, owners []string) error {
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		if _, err := p.DB.Exec(ctx, `INSERT INTO owner_accounts(owner, account)
			VALUES ($1, $2)
			ON CONFLICT(owner) DO UPDATE 
				SET account=$2
				WHERE owner_accounts.account!=$2`,
			owner,
			account,
		); err != nil {
			log.Panic(err)
			return err
		} else if err := p.Log(ctx, idx.OwnerSyncKey, owner, 0); err != nil {
			log.Panic(err)
			return err
		}
	}
	return nil
}
