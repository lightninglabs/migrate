package migrate

import (
	"io"
	nurl "net/url"
	"strings"
	"testing"
)

// countingReader wraps a byte slice and tracks how much has been read.
// It lets tests assert how much of the migration was consumed during
// classification versus execution.
type countingReader struct {
	// data holds the underlying content to be read.
	data []byte

	// read tracks how many bytes have been consumed so far.
	read int
}

// Read copies from the underlying byte slice into p and increments the read
// counter so tests can observe how many bytes were consumed.
func (c *countingReader) Read(p []byte) (int, error) {
	if c.read >= len(c.data) {
		return 0, io.EOF
	}
	n := copy(p, c.data[c.read:])
	c.read += n
	return n, nil
}

func TestSuintPanicsWithNegativeInput(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected suint to panic for -1")
		}
	}()
	suint(-1)
}

func TestSuint(t *testing.T) {
	if u := suint(0); u != 0 {
		t.Fatalf("expected 0, got %v", u)
	}
}

func TestFilterCustomQuery(t *testing.T) {
	n, err := nurl.Parse("foo://host?a=b&x-custom=foo&c=d&ok=y")
	if err != nil {
		t.Fatal(err)
	}
	nx := FilterCustomQuery(n).Query()
	if nx.Get("x-custom") != "" {
		t.Fatalf("didn't expect x-custom")
	}
	if nx.Get("ok") != "y" {
		t.Fatalf("expected ok=y, got %v", nx.Get("ok"))
	}
}

// TestHasSQLMigration verifies detection of executable SQL vs
// comments/whitespace. Data is considered to be executable SQL, if the data
// contains anything else than just comments/whitespace.
func TestHasSQLMigration(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		expectHasSQL bool
	}{
		{
			name:         "empty string",
			data:         "",
			expectHasSQL: false,
		},
		{
			name:         "whitespace and semicolons only",
			data:         "\n;\t;\r\n  ;",
			expectHasSQL: false,
		},
		{
			name:         "only comments",
			data:         "-- comment line\n/* block comment */\n;",
			expectHasSQL: false,
		},
		{
			name:         "bom with no statements",
			data:         "\uFEFF   \n-- just comments\n",
			expectHasSQL: false,
		},
		{
			name:         "sql only",
			data:         "INSERT INTO foo VALUES (1);",
			expectHasSQL: true,
		},
		{
			name:         "sql after comments",
			data:         "-- comment\nSELECT * FROM users; -- trailing comment",
			expectHasSQL: true,
		},
		{
			name:         "sql after block comment",
			data:         "/* header comment */\n\nINSERT INTO foo VALUES (1);",
			expectHasSQL: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := hasSQLMigration(tt.data)
			if err != nil {
				t.Fatalf("hasSQLMigration(%q) unexpected "+
					"error: %v", tt.data, err)
			}
			if got != tt.expectHasSQL {
				t.Fatalf("hasSQLMigration(%q) = %v, want %v",
					tt.data, got, tt.expectHasSQL)
			}
		})
	}
}

// TestClassifyMigrationBodyPeeksAndReplays ensures classifyMigrationBody only
// reads a capped prefix for SQL detection, then replays that prefix followed by
// the remaining body so the database driver still receives the full migration.
func TestClassifyMigrationBodyPeeksAndReplays(t *testing.T) {
	body := "SELECT 1;\n" + strings.Repeat("x", 1024)
	peekCap := 8

	cr := &countingReader{data: []byte(body)}
	migr := &Migration{
		BufferedBody: cr,
		BufferSize:   uint(peekCap),
	}

	hasSQL, err := classifyMigrationBody(migr, 16)
	if err != nil {
		t.Fatalf("classifyMigrationBody error = %v", err)
	}
	if !hasSQL {
		t.Fatalf("expected SQL migration from prefix, got false")
	}

	if cr.read != peekCap {
		t.Fatalf("expected to read %d bytes for classification, "+
			"read %d", peekCap, cr.read)
	}

	replayed, err := io.ReadAll(migr.BufferedBody)
	if err != nil {
		t.Fatalf("reading replayed body: %v", err)
	}
	if string(replayed) != body {
		t.Fatalf("expected replayed body to equal original, got "+
			"len=%d want len=%d", len(replayed), len(body))
	}
	if cr.read != len(body) {
		t.Fatalf("expected underlying reader to be fully consumed, "+
			"read %d want %d", cr.read, len(body))
	}
}

// TestClassifyMigrationBodyMissesDeepSQL shows that if SQL appears beyond the
// peek window, classification returns false, but widening the peek allows
// detection. It also verifies the full body is preserved for execution and the
// underlying reader is fully consumed.
func TestClassifyMigrationBodyMissesDeepSQL(t *testing.T) {
	longComment := "-- " + strings.Repeat("c", 64)
	body := longComment + "\nSELECT 1;"
	peekCap := 16
	wideCap := len(body)

	cr := &countingReader{data: []byte(body)}
	migr := &Migration{
		BufferedBody: cr,
		BufferSize:   uint(peekCap),
	}

	hasSQL, err := classifyMigrationBody(migr, peekCap)
	if err != nil {
		t.Fatalf("classifyMigrationBody error = %v", err)
	}
	if hasSQL {
		t.Fatalf("expected false when SQL is beyond peek window")
	}
	if cr.read != peekCap {
		t.Fatalf("expected to read %d bytes for classification, "+
			"read %d", peekCap, cr.read)
	}

	replayed, err := io.ReadAll(migr.BufferedBody)
	if err != nil {
		t.Fatalf("reading replayed body: %v", err)
	}
	if string(replayed) != body {
		t.Fatalf("expected replayed body to equal original, got "+
			"len=%d want len=%d", len(replayed), len(body))
	}
	if cr.read != len(body) {
		t.Fatalf("expected underlying reader to be fully consumed, "+
			"read %d want %d", cr.read, len(body))
	}

	// Rewind classification with a wider peek to prove SQL is found.
	cr2 := &countingReader{data: []byte(body)}
	migr2 := &Migration{
		BufferedBody: cr2,
		BufferSize:   uint(wideCap),
	}
	hasSQLWide, err := classifyMigrationBody(migr2, wideCap)
	if err != nil {
		t.Fatalf("classifyMigrationBody (wide) error = %v", err)
	}
	if !hasSQLWide {
		t.Fatalf("expected SQL migration when peek window covers the " +
			"body")
	}
	if cr2.read != wideCap {
		t.Fatalf("expected to read %d bytes for wide classification, "+
			"read %d", wideCap, cr2.read)
	}
}
