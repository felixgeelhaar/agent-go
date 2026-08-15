package sandbox

// Contract documents the trust properties every sandboxed pack must uphold.
// Pack tests should cover these cases (see path_test.go and pack-level tests).
//
// Path jail:
//   - Absolute user paths are rejected
//   - ".." escapes that leave BaseDir are rejected
//   - Relative paths resolve under BaseDir
//
// Host allowlist:
//   - Empty allowlist denies all hosts
//   - Exact and "*.suffix" entries are supported
//
// Command allowlist:
//   - Empty allowlist denies all commands
//   - Basename and absolute-path entries are supported
//   - Execution must use argv (exec.Command), never shell interpolation
const ContractVersion = 1
