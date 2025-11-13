package migrate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	nurl "net/url"
	"regexp"
	"strings"
)

// MultiError holds multiple errors.
//
// Deprecated: Use github.com/hashicorp/go-multierror instead
type MultiError struct {
	Errs []error
}

// NewMultiError returns an error type holding multiple errors.
//
// Deprecated: Use github.com/hashicorp/go-multierror instead
func NewMultiError(errs ...error) MultiError {
	compactErrs := make([]error, 0)
	for _, e := range errs {
		if e != nil {
			compactErrs = append(compactErrs, e)
		}
	}
	return MultiError{compactErrs}
}

// Error implements error. Multiple errors are concatenated with 'and's.
func (m MultiError) Error() string {
	var strs = make([]string, 0)
	for _, e := range m.Errs {
		if len(e.Error()) > 0 {
			strs = append(strs, e.Error())
		}
	}
	return strings.Join(strs, " and ")
}

// suint safely converts int to uint
// see https://goo.gl/wEcqof
// see https://goo.gl/pai7Dr
func suint(n int) uint {
	if n < 0 {
		panic(fmt.Sprintf("suint(%v) expects input >= 0", n))
	}
	return uint(n)
}

// FilterCustomQuery filters all query values starting with `x-`
func FilterCustomQuery(u *nurl.URL) *nurl.URL {
	ux := *u
	vx := make(nurl.Values)
	for k, v := range ux.Query() {
		if len(k) <= 1 || k[0:2] != "x-" {
			vx[k] = v
		}
	}
	ux.RawQuery = vx.Encode()
	return &ux
}

// hasSQLMigration checks if the passed data contains executable statements,
// meaning that the data doesn't only contain comments/whitespace or semicolons.
func hasSQLMigration(data string) (bool, error) {
	// Remove Byte Order Mark (BOM) if present in the migration file.
	data = strings.TrimPrefix(data, "\uFEFF")

	// Strip block comments /* ... */ (non-greedy, across lines).
	reBlock := regexp.MustCompile(`(?s)/\*.*?\*/`)
	data = reBlock.ReplaceAllString(data, "")

	// Strip line comments -- ... (to end of line).
	reLine := regexp.MustCompile(`(?m)--[^\n\r]*`)
	data = reLine.ReplaceAllString(data, "")

	// Trim whitespaces.
	data = strings.TrimSpace(data)

	// Remove any semicolons, newlines, tabs, or spaces from the beginning
	// and end of the string.
	data = strings.Trim(data, ";\r\n\t ")

	// If the string still contains any characters, the data likely
	// contains executable statements.
	return len(data) > 0, nil
}

// classifyMigrationBody peeks at a limited prefix of the migration to
// determine if it contains SQL, while preserving the full body for later
// execution by replaying the peeked bytes before the remaining stream.
// The returned bool indicates if the peeked bytes contain SQL migration code.
func classifyMigrationBody(migr *Migration, maxPeekBytes int) (bool, error) {
	peekCap := int(migr.BufferSize)
	if peekCap <= 0 {
		peekCap = int(DefaultBufferSize)
	}
	if peekCap > maxPeekBytes {
		peekCap = maxPeekBytes
	}

	// buf is used to read a small prefix of the migration for
	// classification purposes without holding the whole migration in
	// memory.
	buf := make([]byte, peekCap)

	// Read the data from the migration file, capped by the size of buf.
	n, err := io.ReadFull(migr.BufferedBody, buf)
	switch {
	case err == nil:
	case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF):
		// It's expected that we'll read the end of the file, if the
		// buf size above the size of the migration. In that case, we
		// continue, and only reset the buf to size of n below.
	default:
		return false, fmt.Errorf("read migration body error: %w", err)
	}

	// Set buf to size of n. If the migr.BufferedBody contained more data
	// than the peekCap size, then n will already be the capacity of buf.
	// Else n will be the size of the actual data length.
	buf = buf[:n]

	// Replay the peeked bytes first, then stream the remaining body to the
	// driver.
	migr.BufferedBody = io.MultiReader(
		bytes.NewReader(buf), migr.BufferedBody,
	)

	return hasSQLMigration(string(buf))
}
