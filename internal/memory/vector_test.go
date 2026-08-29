package memory

import (
	"math"
	"reflect"
	"testing"
)

func TestEncodeDecodeVector_Roundtrip(t *testing.T) {
	tests := []struct {
		name   string
		vector []float32
	}{
		{
			name:   "nil vector encodes to empty and decodes to empty",
			vector: nil,
		},
		{
			name:   "empty vector encodes to empty and decodes to empty",
			vector: []float32{},
		},
		{
			name:   "single element vector",
			vector: []float32{1.0},
		},
		{
			name:   "standard float vector",
			vector: []float32{1.0, -0.5, 3.14159, 0.0, -100.25, 42.42},
		},
		{
			name: "768 dimensional vector",
			vector: func() []float32 {
				v := make([]float32, 768)
				for i := range v {
					v[i] = float32(i) * 0.01
				}
				return v
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded := EncodeVector(tt.vector)

			expectedByteLen := len(tt.vector) * 4
			if len(encoded) != expectedByteLen {
				t.Errorf("EncodeVector byte length = %d, want %d", len(encoded), expectedByteLen)
			}

			decoded, err := DecodeVector(encoded)
			if err != nil {
				t.Fatalf("DecodeVector() unexpected error = %v", err)
			}

			if len(tt.vector) == 0 {
				if len(decoded) != 0 {
					t.Errorf("DecodeVector() = %v (len %d), want empty slice", decoded, len(decoded))
				}
				return
			}

			if !reflect.DeepEqual(decoded, tt.vector) {
				t.Errorf("DecodeVector() = %v, want %v", decoded, tt.vector)
			}
		})
	}
}

func TestDecodeVector_MalformedLength(t *testing.T) {
	tests := []struct {
		name  string
		bytes []byte
	}{
		{
			name:  "1 byte",
			bytes: []byte{0x01},
		},
		{
			name:  "2 bytes",
			bytes: []byte{0x01, 0x02},
		},
		{
			name:  "3 bytes",
			bytes: []byte{0x01, 0x02, 0x03},
		},
		{
			name:  "5 bytes",
			bytes: []byte{0x00, 0x00, 0x80, 0x3f, 0x01},
		},
		{
			name:  "7 bytes (non multiple of 4)",
			bytes: []byte{0x00, 0x00, 0x80, 0x3f, 0x00, 0x00, 0x80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decoded, err := DecodeVector(tt.bytes)
			if err == nil {
				t.Fatalf("DecodeVector(%v) expected error on non-multiple-of-4 length, got decoded: %v", tt.bytes, decoded)
			}
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	const epsilon = 1e-5

	tests := []struct {
		name     string
		u        []float32
		v        []float32
		expected float32
	}{
		{
			name:     "identical vectors have similarity 1.0",
			u:        []float32{0.6, 0.8},
			v:        []float32{0.6, 0.8},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors have similarity 0.0",
			u:        []float32{1.0, 0.0},
			v:        []float32{0.0, 1.0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors have similarity -1.0",
			u:        []float32{1.0, 0.0},
			v:        []float32{-1.0, 0.0},
			expected: -1.0,
		},
		{
			name:     "proportional non-unit vectors have similarity 1.0",
			u:        []float32{1.0, 2.0, 3.0},
			v:        []float32{2.0, 4.0, 6.0},
			expected: 1.0,
		},
		{
			name:     "dimension mismatch returns 0.0",
			u:        []float32{1.0, 2.0},
			v:        []float32{1.0, 2.0, 3.0},
			expected: 0.0,
		},
		{
			name:     "zero magnitude vector u returns 0.0",
			u:        []float32{0.0, 0.0},
			v:        []float32{1.0, 1.0},
			expected: 0.0,
		},
		{
			name:     "zero magnitude vector v returns 0.0",
			u:        []float32{1.0, 1.0},
			v:        []float32{0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "both zero magnitude return 0.0",
			u:        []float32{0.0, 0.0},
			v:        []float32{0.0, 0.0},
			expected: 0.0,
		},
		{
			name:     "nil or empty vectors return 0.0",
			u:        nil,
			v:        nil,
			expected: 0.0,
		},
		{
			name:     "empty slice vs empty slice returns 0.0",
			u:        []float32{},
			v:        []float32{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CosineSimilarity(tt.u, tt.v)
			if math.Abs(float64(got-tt.expected)) > epsilon {
				t.Errorf("CosineSimilarity(%v, %v) = %f, want %f (diff %e)", tt.u, tt.v, got, tt.expected, math.Abs(float64(got-tt.expected)))
			}
		})
	}
}
