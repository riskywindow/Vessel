// Package runtime implements the container runtime using Linux primitives.
package runtime

import (
	"fmt"
	"sort"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Seccomp constants (from linux/seccomp.h).
const (
	seccompModeFilter = 2

	seccompRetKillProcess = 0x80000000
	seccompRetErrno       = 0x00050000
	seccompRetAllow       = 0x7fff0000
)

// BPF instruction opcodes.
const (
	bpfLD  = 0x00
	bpfJMP = 0x05
	bpfRET = 0x06
	bpfW   = 0x00
	bpfABS = 0x20
	bpfJEQ = 0x10
	bpfK   = 0x00
)

// Architecture audit constant for x86_64.
const auditArchX86_64 = 0xc000003e

// seccomp_data field offsets.
const (
	offsetNr   = 0 // Syscall number
	offsetArch = 4 // Architecture
)

// blockedSyscalls maps syscall numbers to names for dangerous syscalls.
// These numbers are for x86_64 (amd64).
var blockedSyscalls = map[int]string{
	246: "kexec_load",
	320: "kexec_file_load",
	169: "reboot",
	167: "swapon",
	168: "swapoff",
	321: "bpf",
	248: "add_key",
	250: "keyctl",
	249: "request_key",
	101: "ptrace",
	323: "userfaultfd",
	298: "perf_event_open",
	175: "init_module",
	176: "delete_module",
	313: "finit_module",
	227: "clock_settime",
	261: "clock_adjtime",
}

// bpfStmt creates a BPF statement (no jump).
func bpfStmt(code uint16, k uint32) unix.SockFilter {
	return unix.SockFilter{Code: code, Jt: 0, Jf: 0, K: k}
}

// bpfJump creates a BPF jump instruction.
func bpfJump(code uint16, k uint32, jt, jf uint8) unix.SockFilter {
	return unix.SockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}

// ApplySeccompFilter applies the default seccomp filter to the current process.
// This should be called after filesystem setup but before exec in the container init.
func ApplySeccompFilter() error {
	// NO_NEW_PRIVS is already set by DropCapabilities, but set it again for safety
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("failed to set NO_NEW_PRIVS: %w", err)
	}

	filter := buildSeccompFilter()

	prog := unix.SockFprog{
		Len:    uint16(len(filter)),
		Filter: &filter[0],
	}

	// Use prctl(PR_SET_SECCOMP, SECCOMP_MODE_FILTER, &prog)
	if err := unix.Prctl(unix.PR_SET_SECCOMP, seccompModeFilter, uintptr(unsafe.Pointer(&prog)), 0, 0); err != nil {
		return fmt.Errorf("failed to apply seccomp filter: %w", err)
	}

	return nil
}

// buildSeccompFilter creates a BPF filter that blocks dangerous syscalls.
func buildSeccompFilter() []unix.SockFilter {
	// Sort syscall numbers for deterministic filter output
	sortedSyscalls := make([]int, 0, len(blockedSyscalls))
	for nr := range blockedSyscalls {
		sortedSyscalls = append(sortedSyscalls, nr)
	}
	sort.Ints(sortedSyscalls)

	numBlocked := len(sortedSyscalls)

	// Filter layout:
	//   [0]  Load architecture
	//   [1]  Check arch == x86_64, if yes skip kill, if no fall through
	//   [2]  Kill (wrong arch)
	//   [3]  Load syscall number
	//   [4..4+N-1]  Check each blocked syscall (N checks)
	//   [4+N]  ALLOW (default)
	//   [4+N+1] ERRNO (blocked)

	filter := make([]unix.SockFilter, 0, 4+numBlocked+2)

	// Load architecture
	filter = append(filter, bpfStmt(bpfLD|bpfW|bpfABS, offsetArch))

	// Check architecture - if x86_64 skip to load syscall nr, otherwise kill
	filter = append(filter, bpfJump(bpfJMP|bpfJEQ|bpfK, auditArchX86_64, 1, 0))

	// Kill process if wrong architecture
	filter = append(filter, bpfStmt(bpfRET|bpfK, seccompRetKillProcess))

	// Load syscall number
	filter = append(filter, bpfStmt(bpfLD|bpfW|bpfABS, offsetNr))

	// For each blocked syscall: if match, jump to ERRNO return at the end
	// The ERRNO instruction is at index 4+numBlocked+1 (after all checks and the ALLOW)
	// The ALLOW instruction is at index 4+numBlocked
	for i, nr := range sortedSyscalls {
		// Jump offset to ERRNO: skip remaining checks + ALLOW instruction
		// remaining checks = numBlocked - i - 1, then +1 for ALLOW = numBlocked - i
		jmpToErrno := uint8(numBlocked - i)
		filter = append(filter, bpfJump(bpfJMP|bpfJEQ|bpfK, uint32(nr), jmpToErrno, 0))
	}

	// Allow (default for non-blocked syscalls)
	filter = append(filter, bpfStmt(bpfRET|bpfK, seccompRetAllow))

	// Return ERRNO(EPERM) for blocked syscalls
	filter = append(filter, bpfStmt(bpfRET|bpfK, seccompRetErrno|uint32(unix.EPERM)))

	return filter
}

// GetBlockedSyscalls returns the list of syscall names blocked by the default profile.
func GetBlockedSyscalls() []string {
	names := make([]string, 0, len(blockedSyscalls))
	for _, name := range blockedSyscalls {
		names = append(names, name)
	}
	return names
}
