package scaffold

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// Fault is one scaffolder fault. It states what went wrong and states the
// repair in one sentence. See DX-7.
type Fault struct {
	// Message states the fault.
	Message string `json:"message"`
	// Repair states what to do. It is one sentence.
	Repair string `json:"repair"`
}

// Error returns the message and the repair.
func (f *Fault) Error() string { return fmt.Sprintf("avero new: %s\n  → %s", f.Message, f.Repair) }

// faultOf returns one fault as an error.
func faultOf(message, repair string) error { return &Fault{Message: message, Repair: repair} }

// secret returns a value for AVERO_SECRET of the example environment, so a
// person runs the application at once and no application carries a shared key.
func secret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "change_this_secret_to_the_output_of_openssl_rand_hex_32_0000"
	}
	return hex.EncodeToString(b)
}
