// Package audit provides PDR (Process Decision Record) writing for Neona.
package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/fentz26/neona/internal/models"
	"github.com/fentz26/neona/internal/store"
)

// PDRWriter writes Process Decision Records for audit trails.
type PDRWriter struct {
	store *store.Store
}

// NewPDRWriter creates a new PDR writer.
func NewPDRWriter(s *store.Store) *PDRWriter {
	return &PDRWriter{store: s}
}

// Record writes a PDR entry for a state-mutating action.
func (w *PDRWriter) Record(action string, inputs interface{}, outcome, taskID, details string) (*models.PDREntry, error) {
	inputsHash := hashInputs(inputs)
	return w.store.WritePDR(action, inputsHash, outcome, taskID, details)
}

// hashInputs creates a SHA256 hash of the inputs for reproducibility.
func hashInputs(inputs interface{}) string {
	data, err := json.Marshal(inputs)
	if err != nil {
		return "hash_error"
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
