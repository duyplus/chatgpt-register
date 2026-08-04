package codexreg

// buildAuth constructs auth.json map using accessToken retrieved from browser (access_token mode).
func buildAuth(in Input, accessToken string) map[string]any {
	return map[string]any{
		"auth_mode":    "access_token",
		"access_token": accessToken,
		"email":        in.Email,
	}
}
