package runtime

import (
	"testing"

	"github.com/vessel/vessel/internal/store"
)

func TestValidateResourceLimits(t *testing.T) {
	tests := []struct {
		name      string
		limits    store.ResourceLimits
		wantErr   bool
		wantMem   int64
		wantPids  int64
	}{
		{
			name:     "empty limits get defaults",
			limits:   store.ResourceLimits{},
			wantErr:  false,
			wantMem:  256 * 1024 * 1024, // 256MB default
			wantPids: 512,               // 512 default
		},
		{
			name: "explicit values preserved",
			limits: store.ResourceLimits{
				MemoryMax: 128 * 1024 * 1024,
				PidsMax:   100,
			},
			wantErr:  false,
			wantMem:  128 * 1024 * 1024,
			wantPids: 100,
		},
		{
			name: "negative memory rejected",
			limits: store.ResourceLimits{
				MemoryMax: -1,
			},
			wantErr: true,
		},
		{
			name: "negative PIDs rejected",
			limits: store.ResourceLimits{
				PidsMax: -1,
			},
			wantErr: true,
		},
		{
			name: "negative CPU quota rejected",
			limits: store.ResourceLimits{
				CPUQuota: -1,
			},
			wantErr: true,
		},
		{
			name: "memory too low rejected",
			limits: store.ResourceLimits{
				MemoryMax: 1024, // 1KB - too low
			},
			wantErr: true,
		},
		{
			name: "minimum memory accepted",
			limits: store.ResourceLimits{
				MemoryMax: 4 * 1024 * 1024, // 4MB - minimum
			},
			wantErr:  false,
			wantMem:  4 * 1024 * 1024,
			wantPids: 512, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := tt.limits
			err := ValidateResourceLimits(&limits)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantMem > 0 && limits.MemoryMax != tt.wantMem {
				t.Errorf("MemoryMax = %d, want %d", limits.MemoryMax, tt.wantMem)
			}

			if tt.wantPids > 0 && limits.PidsMax != tt.wantPids {
				t.Errorf("PidsMax = %d, want %d", limits.PidsMax, tt.wantPids)
			}
		})
	}
}
