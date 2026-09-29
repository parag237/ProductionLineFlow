package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"productionlineflow-api/internal/constants"

	"golang.org/x/crypto/argon2"
)

func HashPassword(password string) (string, error) {
	salt := make([]byte, constants.PasswordSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, constants.PasswordIterations, constants.PasswordMemory, constants.PasswordParallelism, constants.PasswordKeyLength)
	encode := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		constants.PasswordMemory,
		constants.PasswordIterations,
		constants.PasswordParallelism,
		encode.EncodeToString(salt),
		encode.EncodeToString(hash),
	), nil
}

func VerifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	parameters := strings.Split(strings.TrimPrefix(parts[3], "m="), ",")
	if len(parameters) != 3 {
		return false
	}
	memory, err := strconv.Atoi(parameters[0])
	if err != nil {
		return false
	}
	iterations, err := strconv.Atoi(strings.TrimPrefix(parameters[1], "t="))
	if err != nil {
		return false
	}
	parallelism, err := strconv.Atoi(strings.TrimPrefix(parameters[2], "p="))
	if err != nil {
		return false
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	actual := argon2.IDKey([]byte(password), salt, uint32(iterations), uint32(memory), uint8(parallelism), uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}
