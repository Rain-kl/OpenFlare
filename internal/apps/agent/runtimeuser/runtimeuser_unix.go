//go:build unix

package runtimeuser

import (
	"fmt"
	"syscall"
)

func init() {
	dropToImpl = func(account *Account) error {
		if err := syscall.Setgid(account.GID); err != nil {
			return fmt.Errorf("setgid %d: %w", account.GID, err)
		}
		if err := syscall.Setuid(account.UID); err != nil {
			return fmt.Errorf("setuid %d: %w", account.UID, err)
		}
		return nil
	}
}
