package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/truthmarket/truth-market/infra/postgres/sqlcgen"
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
	err := r.Q(ctx).CreateUser(ctx, sqlcgen.CreateUserParams{
		ID:            user.ID,
		WalletAddress: textFromString(user.WalletAddress),
		UserType:      string(user.UserType),
		Balance:       user.Balance,
		LockedBalance: user.LockedBalance,
		IsAdmin:       user.IsAdmin,
		CreatedAt:     tstz(user.CreatedAt),
	})
	if err != nil {
		return fmt.Errorf("postgres: create user: %w", err)
	}
	return nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
	row, err := r.Q(ctx).GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New("NOT_FOUND", fmt.Sprintf("user not found: %s", id))
		}
		return nil, fmt.Errorf("postgres: get user by id: %w", err)
	}
	return userFromRow(row), nil
}

func (r *UserRepo) GetByWallet(ctx context.Context, addr string) (*domain.User, error) {
	row, err := r.Q(ctx).GetUserByWallet(ctx, textFromString(addr))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.New("NOT_FOUND", fmt.Sprintf("user not found: %s", addr))
		}
		return nil, fmt.Errorf("postgres: get user by wallet: %w", err)
	}
	return &domain.User{
		ID:            row.ID,
		WalletAddress: stringFromText(row.WalletAddress),
		UserType:      domain.UserType(row.UserType),
		Balance:       row.Balance,
		LockedBalance: row.LockedBalance,
		IsAdmin:       row.IsAdmin,
		CreatedAt:     row.CreatedAt.Time,
	}, nil
}

func (r *UserRepo) UpdateBalance(ctx context.Context, id string, balance, locked decimal.Decimal) error {
	n, err := r.Q(ctx).UpdateUserBalance(ctx, sqlcgen.UpdateUserBalanceParams{
		Balance:       balance,
		LockedBalance: locked,
		ID:            id,
	})
	if err != nil {
		return fmt.Errorf("postgres: update user balance: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("postgres: update user balance: %w", pgx.ErrNoRows)
	}
	return nil
}

func (r *UserRepo) List(ctx context.Context, filter repository.UserFilter) ([]*domain.User, int64, error) {
	userType := pgtype.Text{}
	if filter.UserType != nil {
		userType = textFromString(string(*filter.UserType))
	}
	isAdmin := pgtype.Bool{}
	if filter.IsAdmin != nil {
		isAdmin = pgtype.Bool{Bool: *filter.IsAdmin, Valid: true}
	}

	total, err := r.Q(ctx).CountUsers(ctx, sqlcgen.CountUsersParams{
		UserType: userType,
		IsAdmin:  isAdmin,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list users count: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	rows, err := r.Q(ctx).ListUsers(ctx, sqlcgen.ListUsersParams{
		Limit:    int32(limit),
		Offset:   int32(offset),
		UserType: userType,
		IsAdmin:  isAdmin,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list users query: %w", err)
	}

	users := make([]*domain.User, len(rows))
	for i, row := range rows {
		users[i] = &domain.User{
			ID:            row.ID,
			WalletAddress: stringFromText(row.WalletAddress),
			UserType:      domain.UserType(row.UserType),
			Balance:       row.Balance,
			LockedBalance: row.LockedBalance,
			IsAdmin:       row.IsAdmin,
			CreatedAt:     row.CreatedAt.Time,
		}
	}
	return users, total, nil
}

func userFromRow(r sqlcgen.GetUserByIDRow) *domain.User {
	return &domain.User{
		ID:            r.ID,
		WalletAddress: stringFromText(r.WalletAddress),
		UserType:      domain.UserType(r.UserType),
		Balance:       r.Balance,
		LockedBalance: r.LockedBalance,
		IsAdmin:       r.IsAdmin,
		CreatedAt:     r.CreatedAt.Time,
	}
}
