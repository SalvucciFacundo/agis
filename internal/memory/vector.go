package memory

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// ErrMalformedVector indicates that a vector BLOB's byte length is not a multiple of 4.
var ErrMalformedVector = errors.New("malformed vector BLOB: byte length is not a multiple of 4")

// EncodeVector serializes a float32 slice into raw IEEE 754 LittleEndian bytes for SQLite BLOB storage.
// A nil or empty slice returns an empty byte slice.
func EncodeVector(v []float32) []byte {
	if len(v) == 0 {
		return []byte{}
	}
	buf := make([]byte, len(v)*4)
	for i, val := range v {
		binary.LittleEndian.PutUint32(buf[i*4:(i+1)*4], math.Float32bits(val))
	}
	return buf
}

// DecodeVector deserializes raw IEEE 754 LittleEndian bytes into a float32 slice.
// Returns an error if the byte slice length is not a multiple of 4.
// A nil or empty slice returns an empty float32 slice and nil error.
func DecodeVector(b []byte) ([]float32, error) {
	if len(b) == 0 {
		return []float32{}, nil
	}
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("%w: length %d", ErrMalformedVector, len(b))
	}
	count := len(b) / 4
	res := make([]float32, count)
	for i := 0; i < count; i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		res[i] = math.Float32frombits(bits)
	}
	return res, nil
}

// CosineSimilarity calculates the cosine similarity between two float32 vectors.
// Returns 0.0 if dimensions mismatch, either vector is empty, or either vector has zero magnitude.
func CosineSimilarity(u, v []float32) float32 {
	if len(u) != len(v) || len(u) == 0 {
		return 0.0
	}

	var dot, normU, normV float64
	for i := 0; i < len(u); i++ {
		ui := float64(u[i])
		vi := float64(v[i])
		dot += ui * vi
		normU += ui * ui
		normV += vi * vi
	}

	if normU == 0 || normV == 0 {
		return 0.0
	}

	return float32(dot / (math.Sqrt(normU) * math.Sqrt(normV)))
}
