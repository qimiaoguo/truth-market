package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/pkg/domain"
	apperrors "github.com/truthmarket/truth-market/pkg/errors"
	"github.com/truthmarket/truth-market/pkg/repository"
)

// UserRepo implements repository.UserRepository using PostgreSQL.
type UserRepo struct {
	BaseRepo
}

// NewUserRepo creates a new UserRepo.
func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{BaseRepo: BaseRepo{pool: pool}}
}

// compile-time interface check
var _ repository.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) Create(ctx context.Context, user *domain.User) error {
	q := r.Querier(ctx)

	_, err := q.Exec(ctx,
		`INSERT INTO users (id, wallet_address, user_type, balance, locked_balance, is_admin, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID,
		user.WalletAddress,
		string(user.UserType),
		user.Balance,
		user.LockedBalance,
		user.IsAdmin,
		user.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}

	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
		 FROM users WHERE id = $1`, id)

	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New("NOT_FOUND", fmt.Sprintf("user not found: %s", id))
		}
		return nil, fmt.Errorf("postgres: get user by id: %w", err)
	}

	return u, nil
}

func (r *UserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	q := r.Querier(ctx)

	row := q.QueryRow(ctx,
		`SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
		 FROM users WHERE wallet_address = $1`, addr)

	u, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New("NOT_FOUND", fmt.Sprintf("user not found: %s", addr))
		}
		return nil, fmt.Errorf("postgres: get user by wallet: %w", err)
	}

	return u, nil
}

func (r *UserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	q := r.Querier(ctx)

	tag, err := q.Exec(ctx,
		`UPDATE users SET balance = $1, locked_balance = $2 WHERE id = $3`,
		balance, locked, id)
	if err != nil {
		return fmt.Errorf("postgres: update user balance: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgres: update user balance: %w", pgx.ErrNoRows)
	}

	return nil
}

func (r *UserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	q := r.Querier(ctx)

	var (
		wheres []string
		args   []any
		idx    int
	)

	if filter.UserType != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("user_type = $%d", idx))
		args = append(args, string(*filter.UserType))
	}

	if filter.IsAdmin != nil {
		idx++
		wheres = append(wheres, fmt.Sprintf("is_admin = $%d", idx))
		args = append(args, *filter.IsAdmin)
	}

	where := ""
	if len(wheres) > 0 {
		where = "WHERE " + strings.Join(wheres, " AND ")
	}

	// Count query.
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM users %s", where)

	if err := q.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgres: list users count: %w", err)
	}

	// Data query.
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	idx++
	args = append(args, limit)
	limitPlaceholder := fmt.Sprintf("$%d", idx)

	idx++
	args = append(args, offset)
	offsetPlaceholder := fmt.Sprintf("$%d", idx)

	dataSQL := fmt.Sprintf(
		`SELECT id, wallet_address, user_type, balance, locked_balance, is_admin, created_at
		 FROM users %s ORDER BY created_at DESC LIMIT %s OFFSET %s`,
		where, limitPlaceholder, offsetPlaceholder)

	rows, err := q.Query(ctx, dataSQL, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list users query: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		u, err := scanUserFromRows(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("postgres: list users scan: %w", err)
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("postgres: list users rows: %w", err)
	}

	return users, total, nil
}

// scanUser scans a single user row from pgx.Row.
func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var userType string

	err := row.Scan(
		&u.ID,
		&u.WalletAddress,
		&userType,
		&u.Balance,
		&u.LockedBalance,
		&u.IsAdmin,
		&u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	u.UserType = domain.UserType(userType)
	return &u, nil
}

// scanUserFromRows scans a single user from pgx.Rows.
func scanUserFromRows(rows pgx.Rows) (*domain.User, error) {
	var u domain.User
	var userType string

	err := rows.Scan(
		&u.ID,
		&u.WalletAddress,
		&userType,
		&u.Balance,
		&u.LockedBalance,
		&u.IsAdmin,
		&u.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	u.UserType = domain.UserType(userType)
	return &u, nil
}
