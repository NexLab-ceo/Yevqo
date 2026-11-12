// ============================================================
// REFERRAL / BONUS
// ============================================================

// GetUserByReferralCode — найти пользователя по реферальному коду.
func (db *DB) GetUserByReferralCode(ctx context.Context, code string) (*shared.User, error) {
	query := `
		SELECT id, username, email, password_hash, status,
		       referral_code, referred_by, bonus_devices, max_devices,
		       created_at, last_seen
		FROM users WHERE referral_code = ?`

	var u shared.User
	err := db.conn.QueryRowContext(ctx, query, code).Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Status,
		&u.ReferralCode, &u.ReferredBy, &u.BonusDevices, &u.MaxDevices,
		&u.CreatedAt, &u.LastSeen,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, shared.ErrNotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// AddBonusDevice — начислить бонусные устройства (за друга).
func (db *DB) AddBonusDevice(ctx context.Context, userID int64, n int) error {
	_, err := db.conn.ExecContext(ctx,
		`UPDATE users SET bonus_devices = bonus_devices + ? WHERE id = ?`,
		n, userID,
	)
	return err
}
