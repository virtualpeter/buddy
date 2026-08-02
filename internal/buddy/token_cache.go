package buddy

import "buddy/internal/buddy/tokenauth"

// TokenCacheCleanResult summarizes a tokens/ directory sweep.
type TokenCacheCleanResult = tokenauth.TokenCacheCleanResult

// CleanExpiredTokenCache removes expired or corrupt files under ~/.config/buddy/tokens/.
func CleanExpiredTokenCache() (TokenCacheCleanResult, error) {
	return tokenauth.CleanExpiredTokenCache()
}
