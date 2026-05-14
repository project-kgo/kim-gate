package gateway

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidUserID = errors.New("invalid user id format")

func ParseAppGroup(userID string) (group string, appID string, err error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", "", fmt.Errorf("%w: empty user id", ErrInvalidUserID)
	}

	appID, userPart, found := strings.Cut(userID, ":")
	if !found {
		return "", "", fmt.Errorf("%w: missing app separator", ErrInvalidUserID)
	}
	if strings.Contains(userPart, ":") {
		return "", "", fmt.Errorf("%w: too many app separators", ErrInvalidUserID)
	}

	appID = strings.TrimSpace(appID)
	userPart = strings.TrimSpace(userPart)
	if appID == "" {
		return "", "", fmt.Errorf("%w: app id is required", ErrInvalidUserID)
	}
	if userPart == "" {
		return "", "", fmt.Errorf("%w: user id is required", ErrInvalidUserID)
	}

	return formatAppGroup(appID), appID, nil
}

func formatAppGroup(appID string) string {
	return "app:" + appID
}
