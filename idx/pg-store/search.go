ipackage pgstore

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/chitransh-rockwallet/1sat-indexer/v5/idx"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/jb"
	"github.com/chitransh-rockwallet/1sat-indexer/v5/lib"
)

func (p *PGStore) Search(ctx context.Context, cfg *idx.SearchCfg) (results []*idx.Log, err error) {
	var sqlBuilder strings.Builder
	args := make([]interface{}, 0, 3)
	if cfg.ComparisonType == idx.ComparisonAND && len(cfg.Keys) > 1 {
		// this change is for UTXO API
		sqlBuilder.WriteString(`SELECT min(logs.score) as score, logs.member FROM logs`)
	} else {
		sqlBuilder.WriteString(`SELECT DISTINCT(logs.score), logs.member FROM logs `)
	}
	if cfg.FilterSpent {
		sqlBuilder.WriteString("JOIN txos ON logs.member = txos.outpoint AND txos.spend='' ")
	}

	if len(cfg.Keys) == 1 {
		args = append(args, cfg.Keys[0])
		sqlBuilder.WriteString(`WHERE search_key=$1 `)
	} else {
		args = append(args, cfg.Keys)
		sqlBuilder.WriteString(`WHERE search_key=ANY($1) `)
	}
	if cfg.From != nil {
		args = append(args, *cfg.From)
		if cfg.Reverse {
			sqlBuilder.WriteString(fmt.Sprintf("AND score < $%d ", len(args)))
		} else {
			sqlBuilder.WriteString(fmt.Sprintf("AND score > $%d ", len(args)))
		}
	}

	if cfg.To != nil {
		args = append(args, *cfg.To)
		if cfg.Reverse {
			sqlBuilder.WriteString(fmt.Sprintf("AND score > $%d ", len(args)))
		} else {
			sqlBuilder.WriteString(fmt.Sprintf("AND score < $%d ", len(args)))
		}
	}

	if cfg.ComparisonType == idx.ComparisonAND && len(cfg.Keys) > 1 {
		args = append(args, len(cfg.Keys))
		sqlBuilder.WriteString("GROUP BY logs.member ")
		sqlBuilder.WriteString(fmt.Sprintf("HAVING COUNT(1) = $%d ", len(args)))
	} else {
		sqlBuilder.WriteString("GROUP BY logs.score, logs.member")
	}

	if cfg.Reverse {
		sqlBuilder.WriteString("ORDER BY score DESC ")
	} else {
		sqlBuilder.WriteString("ORDER BY score ASC ")
	}

	if cfg.Limit > 0 {
		args = append(args, cfg.Limit)
		sqlBuilder.WriteString(fmt.Sprintf("LIMIT $%d ", len(args)))
	}

	sql := sqlBuilder.String()
	var start time.Time
	if cfg.Verbose {
		log.Println(sql, args)
		start = time.Now()

	}
	if rows, err := p.DB.Query(ctx, sql, args...); err != nil {
		return nil, err
	} else {
		if cfg.Verbose {
			log.Println("Query time", time.Since(start))
		}
		defer rows.Close()
		results = make([]*idx.Log, 0, cfg.Limit)
		for rows.Next() {
			var result idx.Log
			if err = rows.Scan(&result.Score, &result.Member); err != nil {
				return nil, err
			}
			if cfg.RefreshSpends {
				log.Println("Refreshing spends", result.Member)
				if spend, err := jb.GetSpend(result.Member); err != nil {
					return nil, err
				} else if spend != "" {
					p.SetNewSpend(ctx, result.Member, spend)
					continue
				}
			}
			results = append(results, &result)
		}
	}
	if cfg.Verbose {
		log.Println("Results", len(results))
	}
	return results, nil
}

func (p *PGStore) SearchMembers(ctx context.Context, cfg *idx.SearchCfg) (results []string, err error) {
	if items, err := p.Search(ctx, cfg); err != nil {
		return nil, err
	} else {
		members := make([]string, 0, len(items))
		for _, item := range items {
			members = append(members, item.Member)
		}
		return members, nil
	}
}

func (p *PGStore) SearchOutpoints(ctx context.Context, cfg *idx.SearchCfg) (results []string, err error) {
	if items, err := p.Search(ctx, cfg); err != nil {
		return nil, err
	} else {
		outpoints := make([]string, 0, len(items))
		for _, item := range items {
			if len(item.Member) < 65 {
				continue
			}
			outpoints = append(outpoints, item.Member)
		}
		return outpoints, nil
	}
}

func (p *PGStore) SearchTxos(ctx context.Context, cfg *idx.SearchCfg) (txos []*idx.Txo, err error) {
	if cfg.IncludeTxo {
		var outpoints []string
		if outpoints, err = p.SearchOutpoints(ctx, cfg); err != nil {
			return nil, err
		}
		if txos, err = p.LoadTxos(ctx, outpoints, cfg.IncludeTags, cfg.IncludeScript, cfg.IncludeSpend); err != nil {
			return nil, err
		}
	} else {
		if results, err := p.Search(ctx, cfg); err != nil {
			return nil, err
		} else {
			txos = make([]*idx.Txo, 0, len(results))
			for _, result := range results {
				txo := &idx.Txo{
					Height: uint32(result.Score / 1000000000),
					Idx:    uint64(result.Score) % 1000000000,
					Score:  result.Score,
					Data:   make(map[string]*idx.IndexData),
				}
				if txo.Outpoint, err = lib.NewOutpointFromString(result.Member); err != nil {
					return nil, err
				} else if txo.Data, err = p.LoadData(ctx, txo.Outpoint.String(), cfg.IncludeTags); err != nil {
					return nil, err
				} else if txo.Data == nil {
					txo.Data = make(idx.IndexDataMap)
				}
				if cfg.IncludeScript {
					if err := txo.LoadScript(ctx); err != nil {
						return nil, err
					}
				}
				txos = append(txos, txo)
			}
		}
	}

	return txos, nil
}

func (p *PGStore) SearchBalance(ctx context.Context, cfg *idx.SearchCfg) (balance uint64, err error) {
	cfg.FilterSpent = true
	if outpoints, err := p.SearchOutpoints(ctx, cfg); err != nil {
		return 0, err
	} else if txos, err := p.LoadTxos(ctx, outpoints, nil, false, false); err != nil {
		return 0, err
	} else {
		for _, txo := range txos {
			if txo.Satoshis != nil {
				balance += *txo.Satoshis
			}
		}
	}

	return
}

func (p *PGStore) SearchTxns(ctx context.Context, cfg *idx.SearchCfg) (txns []*lib.TxResult, err error) {
	results := make([]*lib.TxResult, 0, cfg.Limit)
	if activity, err := p.Search(ctx, cfg); err != nil {
		return nil, err
	} else {
		for _, item := range activity {
			var txid string
			var out *uint32
			if len(item.Member) == 64 {
				txid = item.Member
			} else if outpoint, err := lib.NewOutpointFromString(item.Member); err != nil {
				return nil, err
			} else {
				txid = outpoint.TxidHex()
				vout := outpoint.Vout()
				out = &vout
			}
			var result *lib.TxResult
			height := uint32(item.Score / 1000000000)
			result = &lib.TxResult{
				Txid:    txid,
				Height:  height,
				Idx:     uint64(item.Score) % 1000000000,
				Outputs: lib.NewOutputMap(),
				Score:   item.Score,
			}
			results = append(results, result)
			if cfg.IncludeRawtx {
				if result.Rawtx, err = jb.LoadRawtx(ctx, txid); err != nil {
					return nil, err
				}
			}
			if out != nil {
				result.Outputs[*out] = struct{}{}
			}
		}
	}
	return results, nil
}

func (p *PGStore) Balance(ctx context.Context, key string) (balance int64, err error) {
	// row := p.DB.QueryRow(ctx, `SELECT SUM(satoshis)
	// 	FROM logs
	// 	WHERE key = $1`,
	// 	key,
	// )
	// if err = row.Scan(&balance); err != nil {
	// 	return 0, err
	// }
	return
}

func (p *PGStore) CountMembers(ctx context.Context, key string) (count uint64, err error) {
	row := p.DB.QueryRow(ctx, `SELECT COUNT(1)
		FROM logs
		WHERE key = $1`,
		key,
	)
	if err = row.Scan(&count); err != nil {
		return 0, err
	}
	return
}

