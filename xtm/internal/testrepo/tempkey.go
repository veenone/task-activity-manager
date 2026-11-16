package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
)

// nextFreeKey counts up from 1 and returns the first prefix+N the probe query
// does not find. The probe's last placeholder takes the candidate; args fill
// the ones before it. what names the key in the error.
func nextFreeKey(tx *sql.Tx, what, prefix, probe string, args ...any) (string, error) {
	for n := 1; ; n++ {
		key := fmt.Sprintf("%s%d", prefix, n)
		var one int
		err := tx.QueryRow(probe, append(args[:len(args):len(args)], key)...).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return key, nil
		}
		if err != nil {
			return "", fmt.Errorf("probe temp %s: %w", what, err)
		}
	}
}
