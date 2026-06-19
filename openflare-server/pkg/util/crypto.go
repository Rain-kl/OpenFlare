// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package util provides generic utility functions.
package util

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	aesKeyLength = 32

	errInvalidSignKey         = "invalid sign key: %w"
	errSignKeyLengthInvalid   = "sign key must be 32 bytes (64 hex characters)"
	errCreateCipherFailed     = "failed to create cipher: %w"
	errCreateGCMFailed        = "failed to create GCM: %w"
	errGenerateNonceFailed    = "failed to generate nonce: %w"
	errDecodeCiphertextFailed = "failed to decode ciphertext: %w"
	errCiphertextTooShort     = "ciphertext too short"
	errDecryptFailed          = "failed to decrypt: %w"
)

// Encrypt 使用 SignKey 加密字符串数据
// signKey: 64 字符 hex 编码的密钥（对应 32 字节，用于 AES-256）
// plaintext: 要加密的明文字符串
// return: base64 编码的密文
func Encrypt(signKey string, plaintext string) (string, error) {
	return encryptBytes(signKey, []byte(plaintext))
}

// Decrypt 使用 SignKey 解密字符串数据
// signKey: 64 字符 hex 编码的密钥（对应 32 字节，用于 AES-256）
// ciphertext: base64 编码的密文
// return: 解密后的明文字符串
func Decrypt(signKey string, ciphertext string) (string, error) {
	plaintext, err := decryptBytes(signKey, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// encryptBytes 加密函数，处理字节数据
func encryptBytes(signKey string, plaintext []byte) (string, error) {
	// 将 hex 编码的密钥转换为字节
	key, err := hex.DecodeString(signKey)
	if err != nil {
		return "", fmt.Errorf(errInvalidSignKey, err)
	}
	if len(key) != aesKeyLength {
		return "", errors.New(errSignKeyLengthInvalid)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf(errCreateCipherFailed, err)
	}

	// 使用 GCM 模式（Galois/Counter Mode）
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf(errCreateGCMFailed, err)
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf(errGenerateNonceFailed, err)
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// 返回 base64 编码的密文
	return Base64Encode(ciphertext), nil
}

// decryptBytes 解密函数，处理字节数据
func decryptBytes(signKey string, ciphertext string) ([]byte, error) {
	// 将 hex 编码的密钥转换为字节
	key, err := hex.DecodeString(signKey)
	if err != nil {
		return nil, fmt.Errorf(errInvalidSignKey, err)
	}
	if len(key) != aesKeyLength {
		return nil, errors.New(errSignKeyLengthInvalid)
	}

	// 解码 base64 密文
	data, err := Base64Decode(ciphertext)
	if err != nil {
		return nil, fmt.Errorf(errDecodeCiphertextFailed, err)
	}

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf(errCreateCipherFailed, err)
	}

	// 使用 GCM 模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf(errCreateGCMFailed, err)
	}

	// 提取 nonce
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New(errCiphertextTooShort)
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return nil, fmt.Errorf(errDecryptFailed, err)
	}

	return plaintext, nil
}

// Base64Encode Base64编码
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode Base64解码
func Base64Decode(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}

// Ed25519Verify 验证 Ed25519 签名
// publicKey: 32 字节的公钥（已解码的二进制格式）
// message: 待验证的原始消息
// signature: 64 字节的签名（已解码的二进制格式）
// return: 签名是否有效
func Ed25519Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}

	if len(signature) != ed25519.SignatureSize {
		return false
	}

	return ed25519.Verify(publicKey, message, signature)
}
