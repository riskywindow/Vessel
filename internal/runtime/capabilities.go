// Package runtime implements the container runtime using Linux primitives.
package runtime

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Linux capability constants.
// From linux/capability.h - these are the capability numbers.
const (
	CAP_CHOWN            = 0
	CAP_DAC_OVERRIDE     = 1
	CAP_DAC_READ_SEARCH  = 2
	CAP_FOWNER           = 3
	CAP_FSETID           = 4
	CAP_KILL             = 5
	CAP_SETGID           = 6
	CAP_SETUID           = 7
	CAP_SETPCAP          = 8
	CAP_LINUX_IMMUTABLE  = 9
	CAP_NET_BIND_SERVICE = 10
	CAP_NET_BROADCAST    = 11
	CAP_NET_ADMIN        = 12
	CAP_NET_RAW          = 13
	CAP_IPC_LOCK         = 14
	CAP_IPC_OWNER        = 15
	CAP_SYS_MODULE       = 16
	CAP_SYS_RAWIO        = 17
	CAP_SYS_CHROOT       = 18
	CAP_SYS_PTRACE       = 19
	CAP_SYS_PACCT        = 20
	CAP_SYS_ADMIN        = 21
	CAP_SYS_BOOT         = 22
	CAP_SYS_NICE         = 23
	CAP_SYS_RESOURCE     = 24
	CAP_SYS_TIME         = 25
	CAP_SYS_TTY_CONFIG   = 26
	CAP_MKNOD            = 27
	CAP_LEASE            = 28
	CAP_AUDIT_WRITE      = 29
	CAP_AUDIT_CONTROL    = 30
	CAP_SETFCAP          = 31
	CAP_MAC_OVERRIDE     = 32
	CAP_MAC_ADMIN        = 33
	CAP_SYSLOG           = 34
	CAP_WAKE_ALARM       = 35
	CAP_BLOCK_SUSPEND    = 36
	CAP_AUDIT_READ       = 37
	CAP_PERFMON          = 38
	CAP_BPF              = 39
	CAP_CHECKPOINT_RESTORE = 40

	// CAP_LAST_CAP should be updated when new capabilities are added to the kernel.
	CAP_LAST_CAP = 40
)

// defaultCaps is the set of capabilities to keep.
// This matches Docker's default capability set, which is well-tested.
var defaultCaps = map[int]bool{
	CAP_NET_BIND_SERVICE: true, // Bind to ports < 1024
	CAP_CHOWN:            true, // Change file ownership
	CAP_DAC_OVERRIDE:     true, // Bypass file permission checks
	CAP_FOWNER:           true, // Bypass permission checks for file owner operations
	CAP_FSETID:           true, // Don't clear setuid/setgid bits on file modification
	CAP_KILL:             true, // Send signals to any process
	CAP_SETGID:           true, // Change GID
	CAP_SETUID:           true, // Change UID
	CAP_SETFCAP:          true, // Set file capabilities
	CAP_NET_RAW:          true, // Use raw sockets (ping)
	CAP_SYS_CHROOT:       true, // Use chroot
	CAP_MKNOD:            true, // Create device nodes
	CAP_AUDIT_WRITE:      true, // Write to audit log
	CAP_SETPCAP:          true, // Modify process capabilities
}

// DropCapabilities drops all capabilities except those in the default set.
// This should be called after entering the namespace but before exec-ing the container process.
func DropCapabilities() error {
	// First, set PR_SET_NO_NEW_PRIVS to prevent privilege escalation through setuid binaries
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("failed to set NO_NEW_PRIVS: %w", err)
	}

	// Drop capabilities from the bounding set
	for cap := 0; cap <= CAP_LAST_CAP; cap++ {
		if defaultCaps[cap] {
			// Keep this capability
			continue
		}

		// Drop this capability from the bounding set
		if err := unix.Prctl(unix.PR_CAPBSET_DROP, uintptr(cap), 0, 0, 0); err != nil {
			// Capability might not exist on this kernel, that's OK
			if err != unix.EINVAL {
				return fmt.Errorf("failed to drop capability %d: %w", cap, err)
			}
		}
	}

	return nil
}

// capabilityNames maps capability numbers to names (for debugging/logging).
var capabilityNames = map[int]string{
	CAP_CHOWN:            "CAP_CHOWN",
	CAP_DAC_OVERRIDE:     "CAP_DAC_OVERRIDE",
	CAP_DAC_READ_SEARCH:  "CAP_DAC_READ_SEARCH",
	CAP_FOWNER:           "CAP_FOWNER",
	CAP_FSETID:           "CAP_FSETID",
	CAP_KILL:             "CAP_KILL",
	CAP_SETGID:           "CAP_SETGID",
	CAP_SETUID:           "CAP_SETUID",
	CAP_SETPCAP:          "CAP_SETPCAP",
	CAP_LINUX_IMMUTABLE:  "CAP_LINUX_IMMUTABLE",
	CAP_NET_BIND_SERVICE: "CAP_NET_BIND_SERVICE",
	CAP_NET_BROADCAST:    "CAP_NET_BROADCAST",
	CAP_NET_ADMIN:        "CAP_NET_ADMIN",
	CAP_NET_RAW:          "CAP_NET_RAW",
	CAP_IPC_LOCK:         "CAP_IPC_LOCK",
	CAP_IPC_OWNER:        "CAP_IPC_OWNER",
	CAP_SYS_MODULE:       "CAP_SYS_MODULE",
	CAP_SYS_RAWIO:        "CAP_SYS_RAWIO",
	CAP_SYS_CHROOT:       "CAP_SYS_CHROOT",
	CAP_SYS_PTRACE:       "CAP_SYS_PTRACE",
	CAP_SYS_PACCT:        "CAP_SYS_PACCT",
	CAP_SYS_ADMIN:        "CAP_SYS_ADMIN",
	CAP_SYS_BOOT:         "CAP_SYS_BOOT",
	CAP_SYS_NICE:         "CAP_SYS_NICE",
	CAP_SYS_RESOURCE:     "CAP_SYS_RESOURCE",
	CAP_SYS_TIME:         "CAP_SYS_TIME",
	CAP_SYS_TTY_CONFIG:   "CAP_SYS_TTY_CONFIG",
	CAP_MKNOD:            "CAP_MKNOD",
	CAP_LEASE:            "CAP_LEASE",
	CAP_AUDIT_WRITE:      "CAP_AUDIT_WRITE",
	CAP_AUDIT_CONTROL:    "CAP_AUDIT_CONTROL",
	CAP_SETFCAP:          "CAP_SETFCAP",
	CAP_MAC_OVERRIDE:     "CAP_MAC_OVERRIDE",
	CAP_MAC_ADMIN:        "CAP_MAC_ADMIN",
	CAP_SYSLOG:           "CAP_SYSLOG",
	CAP_WAKE_ALARM:       "CAP_WAKE_ALARM",
	CAP_BLOCK_SUSPEND:    "CAP_BLOCK_SUSPEND",
	CAP_AUDIT_READ:       "CAP_AUDIT_READ",
	CAP_PERFMON:          "CAP_PERFMON",
	CAP_BPF:              "CAP_BPF",
	CAP_CHECKPOINT_RESTORE: "CAP_CHECKPOINT_RESTORE",
}

// GetCapabilityName returns the name of a capability number.
func GetCapabilityName(cap int) string {
	if name, ok := capabilityNames[cap]; ok {
		return name
	}
	return fmt.Sprintf("CAP_%d", cap)
}

// GetDefaultCapabilities returns the list of capability names that are kept by default.
func GetDefaultCapabilities() []string {
	caps := make([]string, 0, len(defaultCaps))
	for cap := range defaultCaps {
		caps = append(caps, GetCapabilityName(cap))
	}
	return caps
}

// GetDroppedCapabilities returns the list of capability names that are dropped by default.
func GetDroppedCapabilities() []string {
	var caps []string
	for cap := 0; cap <= CAP_LAST_CAP; cap++ {
		if !defaultCaps[cap] {
			caps = append(caps, GetCapabilityName(cap))
		}
	}
	return caps
}
