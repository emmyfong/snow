package credential

import "github.com/emmyfong/snow/internal/provider"

// A Store gives providers their keys without either package importing the
// other.
var _ provider.KeySource = (*Store)(nil)
