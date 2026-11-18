package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func (s *Server) SeedAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if m := validateEmail(email); m != "" {
		return fmt.Errorf("ADMIN_EMAIL is not a valid email")
	}
	if m := validatePassword(password); m != "" {
		return fmt.Errorf("ADMIN_PASSWORD does not meet the password policy")
	}

	_, err := s.store.FindUserByEmail(ctx, email)
	if err == nil {
		s.logger.Info("admin account already exists")
		return nil
	}
	if !errors.Is(err, ErrNotFound) {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	admin := &User{
		FirstName:         "Admin",
		LastName:          "Admin",
		Email:             email,
		PasswordHash:      string(hash),
		Role:              RoleAdmin,
		Status:            StatusActive,
		QuotaBytes:        s.defaultQuota,
		PasswordChangedAt: now,
		CreatedAt:         now,
	}
	if err := s.store.CreateUser(ctx, admin); err != nil {
		return err
	}
	s.logger.Info("admin account created", "userId", admin.ID.Hex())
	return nil
}
