// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/migrator"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/platform/bootstrap"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

var (
	usernameFlag string
	passwordFlag string
)

const (
	passwdCharset         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	defaultPasswordLength = 16
)

func generateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i, b := range bytes {
		bytes[i] = passwdCharset[int(b)%len(passwdCharset)]
	}
	return string(bytes), nil
}

var resetPasswdCmd = &cobra.Command{
	Use:   "reset-passwd",
	Short: "Reset the password for a specified account",
	PreRun: func(_ *cobra.Command, _ []string) {
		migrator.Migrate()
	},
	Run: func(_ *cobra.Command, _ []string) {
		ctx := context.Background()
		runBootstrap(bootstrap.Options{})

		var username string
		if usernameFlag != "" {
			username = usernameFlag
		} else {
			fmt.Print("Enter username: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Fatalf("Failed to read username: %v\n", err)
			}
			username = strings.TrimSpace(input)
			if username == "" {
				log.Fatal("Username must not be empty\n")
			}
		}

		user, err := repository.GetUserByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				log.Fatalf("Error: user '%s' was not found\n", username)
			}
			log.Fatalf("Failed to query user: %v\n", err)
		}

		var password string
		if passwordFlag != "" {
			password = passwordFlag
		} else {
			password, err = generateRandomPassword(defaultPasswordLength)
			if err != nil {
				log.Fatalf("Failed to generate random password: %v\n", err)
			}
		}

		if err := user.SetEncryptedPassword(password); err != nil {
			log.Fatalf("Failed to encrypt password: %v\n", err)
		}

		err = db.DB(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&user).Update("password", user.Password).Error; err != nil {
				return err
			}

			// Invalidate existing tokens
			var tokens []model.AccessToken
			if err := tx.Where("user_id = ?", user.ID).Find(&tokens).Error; err == nil {
				for _, token := range tokens {
					oauth.InvalidateCachedToken(ctx, token.TokenHash)
				}
			}

			return tx.Where("user_id = ?", user.ID).Delete(&model.AccessToken{}).Error
		})
		if err != nil {
			log.Fatalf("Failed to reset password: %v\n", err)
		}

		oauth.InvalidateCachedUser(ctx, user.ID)

		fmt.Println("Password reset successfully")
		fmt.Printf("Username: %s\n", user.Username)
		fmt.Printf("New password: %s\n", password)
	},
}

func init() {
	resetPasswdCmd.Flags().StringVar(&usernameFlag, "user", "", "Username of the account whose password will be reset")
	resetPasswdCmd.Flags().StringVar(&passwordFlag, "password", "", "New password; generated randomly if omitted")
	rootCmd.AddCommand(resetPasswdCmd)
}
