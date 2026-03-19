// Run result queries.
// source: run_result.sql

package db

import (
	"context"
)

const setRunResult = `-- name: SetRunResult :exec
UPDATE runs SET result = $2, updated_at = now() WHERE id = $1
`

func (q *Queries) SetRunResult(ctx context.Context, id string, result []byte) error {
	_, err := q.db.Exec(ctx, setRunResult, id, result)
	return err
}

const getRunResult = `-- name: GetRunResult :one
SELECT id, result FROM runs WHERE id = $1
`

type RunResult struct {
	ID     string `json:"id"`
	Result []byte `json:"result"`
}

func (q *Queries) GetRunResult(ctx context.Context, id string) (RunResult, error) {
	row := q.db.QueryRow(ctx, getRunResult, id)
	var r RunResult
	err := row.Scan(&r.ID, &r.Result)
	return r, err
}
