package persistence

import "fmt"

// Minimum padding, not a length cap. Numbers are provider-local, never credentials.
func orderReference(number int64) string { return fmt.Sprintf("O-%06d", number) }
