package store

import "context"

// UpsertDevice registers a push token, re-assigning it when another user last held it.
func (s *Store) UpsertDevice(ctx context.Context, userID, token, platform string) error {
	return s.exec(ctx,
		`INSERT INTO devices (id, user_id, push_token, platform, created_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(push_token) DO UPDATE SET user_id = excluded.user_id, platform = excluded.platform`,
		newID(), userID, token, platform, toMs(now()))
}

func (s *Store) DeleteDevice(ctx context.Context, token, userID string) error {
	return s.exec(ctx, `DELETE FROM devices WHERE push_token = ? AND user_id = ?`, token, userID)
}

func (s *Store) ListDeviceTokens(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT push_token FROM devices WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) DeleteDevicesByTokens(ctx context.Context, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	return s.exec(ctx, `DELETE FROM devices WHERE push_token IN (`+placeholders(len(tokens))+`)`, toAny(tokens)...)
}
