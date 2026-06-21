package user

import (
	"time"

	"file-storage-server/internal/domain"
)

type RegisterUserDTO struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginUserDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthTokenDTO struct {
	Token                 string  `json:"token"`
	AccessToken           string  `json:"accessToken"`
	RefreshToken          string  `json:"refreshToken"`
	AccessTokenExpiresAt  string  `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt string  `json:"refreshTokenExpiresAt"`
	User                  UserDTO `json:"user"`
}

type UserDTO struct {
	ID          int64           `json:"id"`
	Username    string          `json:"username"`
	Email       string          `json:"email"`
	Role        domain.UserRole `json:"role"`
	AvatarPath  *string         `json:"avatarPath,omitempty"`
	DateOfBirth *time.Time      `json:"dateOfBirth,omitempty"`
	Theme       string          `json:"theme"`
	Language    string          `json:"language"`
}

type CreateUserParams struct {
	Username     string
	Email        string
	PasswordHash string
	Role         domain.UserRole
}

type UpdateUserRoleParams struct {
	UserID int64
	Role   domain.UserRole
}
