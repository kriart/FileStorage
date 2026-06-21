package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"file-storage-server/internal/config"
	"file-storage-server/internal/domain"
	"file-storage-server/internal/repository"
	"file-storage-server/internal/user"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	users           user.Repository
	refreshTokens   RefreshTokenRepository
	secret          []byte
	tokenTTL        time.Duration
	refreshTokenTTL time.Duration
	issuer          string
	now             func() time.Time
	jwtParser       *jwt.Parser
}

type CurrentUser struct {
	ID         int64           `json:"id"`
	Role       domain.UserRole `json:"role"`
	Email      string          `json:"email,omitempty"`
	Username   string          `json:"username,omitempty"`
	AvatarPath *string         `json:"avatarPath,omitempty"`
	Theme      string          `json:"theme,omitempty"`
	Language   string          `json:"language,omitempty"`
}

type Claims struct {
	UserID int64           `json:"uid"`
	Role   domain.UserRole `json:"role"`
	jwt.RegisteredClaims
}

func NewService(users user.Repository, refreshTokens RefreshTokenRepository, cfg config.AuthConfig) *Service {
	return &Service{
		users:           users,
		refreshTokens:   refreshTokens,
		secret:          []byte(cfg.JWTSecret),
		tokenTTL:        cfg.TokenTTL,
		refreshTokenTTL: cfg.RefreshTokenTTL,
		issuer:          "file-storage-server",
		now:             time.Now,
		jwtParser:       jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})),
	}
}

func (s *Service) Register(ctx context.Context, dto user.RegisterUserDTO) (*user.AuthTokenDTO, error) {
	username := strings.TrimSpace(dto.Username)
	email := strings.ToLower(strings.TrimSpace(dto.Email))
	password := dto.Password

	if username == "" || email == "" || password == "" || !strings.Contains(email, "@") || len(password) < 8 {
		return nil, repository.ErrInvalidInput
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	created, err := s.users.Create(ctx, user.CreateUserParams{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         domain.UserRoleUser,
	})
	if err != nil {
		return nil, err
	}

	return s.authToken(ctx, created)
}

func (s *Service) Login(ctx context.Context, dto user.LoginUserDTO) (*user.AuthTokenDTO, error) {
	email := strings.ToLower(strings.TrimSpace(dto.Email))
	if email == "" || dto.Password == "" {
		return nil, repository.ErrUnauthorized
	}

	found, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrUnauthorized
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(found.PasswordHash), []byte(dto.Password)); err != nil {
		return nil, repository.ErrUnauthorized
	}

	return s.authToken(ctx, found)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*user.AuthTokenDTO, error) {
	if s.refreshTokens == nil {
		return nil, repository.ErrUnauthorized
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, repository.ErrUnauthorized
	}

	now := s.now().UTC()
	newRefreshToken, err := generateRefreshToken()
	if err != nil {
		return nil, err
	}
	newRefreshExpiresAt := now.Add(s.refreshTokenTTL)
	stored, err := s.refreshTokens.RotateRefreshToken(ctx, hashRefreshToken(refreshToken), hashRefreshToken(newRefreshToken), newRefreshExpiresAt, now)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrUnauthorized) {
			return nil, repository.ErrUnauthorized
		}
		return nil, err
	}

	found, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrUnauthorized
		}
		return nil, err
	}
	return s.authTokenWithRefresh(found, newRefreshToken, newRefreshExpiresAt)
}

func (s *Service) RevokeRefreshToken(ctx context.Context, refreshToken string) error {
	if s.refreshTokens == nil {
		return nil
	}
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	stored, err := s.refreshTokens.GetRefreshTokenByHash(ctx, hashRefreshToken(refreshToken))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	return s.refreshTokens.RevokeRefreshToken(ctx, stored.ID)
}

func (s *Service) CurrentUser(ctx context.Context, token string) (*CurrentUser, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, repository.ErrUnauthorized
	}

	claims := new(Claims)
	parsed, err := s.jwtParser.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, repository.ErrUnauthorized
	}

	if claims.UserID <= 0 {
		return nil, repository.ErrUnauthorized
	}

	found, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrUnauthorized
		}
		return nil, err
	}

	return &CurrentUser{
		ID:         found.ID,
		Role:       found.Role,
		Email:      found.Email,
		Username:   found.Username,
		AvatarPath: found.AvatarPath,
		Theme:      found.Theme,
		Language:   found.Language,
	}, nil
}

func (s *Service) authToken(ctx context.Context, userModel *domain.User) (*user.AuthTokenDTO, error) {
	var err error
	refreshToken := ""
	refreshExpiresAt := time.Time{}
	if s.refreshTokens != nil {
		refreshToken, err = generateRefreshToken()
		if err != nil {
			return nil, err
		}
		refreshExpiresAt = s.now().UTC().Add(s.refreshTokenTTL)
		if _, err := s.refreshTokens.CreateRefreshToken(ctx, CreateRefreshTokenParams{
			UserID:    userModel.ID,
			TokenHash: hashRefreshToken(refreshToken),
			ExpiresAt: refreshExpiresAt,
		}); err != nil {
			return nil, err
		}
	}

	return s.authTokenWithRefresh(userModel, refreshToken, refreshExpiresAt)
}

func (s *Service) authTokenWithRefresh(userModel *domain.User, refreshToken string, refreshExpiresAt time.Time) (*user.AuthTokenDTO, error) {
	accessToken, accessExpiresAt, err := s.signToken(userModel)
	if err != nil {
		return nil, err
	}

	return &user.AuthTokenDTO{
		Token:                 accessToken,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresAt:  accessExpiresAt.Format(time.RFC3339),
		RefreshTokenExpiresAt: refreshExpiresAt.Format(time.RFC3339),
		User: user.UserDTO{
			ID:          userModel.ID,
			Username:    userModel.Username,
			Email:       userModel.Email,
			Role:        userModel.Role,
			AvatarPath:  userModel.AvatarPath,
			DateOfBirth: userModel.DateOfBirth,
			Theme:       userModel.Theme,
			Language:    userModel.Language,
		},
	}, nil
}

func (s *Service) signToken(userModel *domain.User) (string, time.Time, error) {
	now := s.now().UTC()
	expiresAt := now.Add(s.tokenTTL)
	claims := Claims{
		UserID: userModel.ID,
		Role:   userModel.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userModel.ID, 10),
			Issuer:    s.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	return signed, expiresAt, err
}

func generateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
