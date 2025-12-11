package migrate

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/database"
	dStub "github.com/golang-migrate/migrate/v4/database/stub"
	"github.com/golang-migrate/migrate/v4/source"
	sStub "github.com/golang-migrate/migrate/v4/source/stub"
)

// sourceStubMigrations hold the following migrations:
// u = up migration, d = down migration, n = version
//
//	|  1  |  -  |  3  |  4  |  5  |  -  |  7  |
//	| u d |  -  | u   | u d |   d |  -  | u d |
var sourceStubMigrations *source.Migrations

// sourceProgrammaticMigrations hold the following migrations:
// u = up migration, d = down migration, n = version, SQL = SQL migration,
// PGM = Programmatic migration
//
//	|  1  |  2  |  3  |  4  |  5  |  -  |  7  |
//	| SQL | PGM | SQL | SQL | SQL |  -  | PGM |
//	| u d | u d | u   | u d |   d |  -  | u   |
var sourceProgrammaticMigrations *source.Migrations

const (
	srcDrvNameStub = "stub"
	dbDrvNameStub  = "stub"
)

func init() {
	sourceStubMigrations = source.NewMigrations()
	sourceStubMigrations.Append(&source.Migration{Version: 1, Direction: source.Up, Identifier: "CREATE 1"})
	sourceStubMigrations.Append(&source.Migration{Version: 1, Direction: source.Down, Identifier: "DROP 1"})
	sourceStubMigrations.Append(&source.Migration{Version: 3, Direction: source.Up, Identifier: "CREATE 3"})
	sourceStubMigrations.Append(&source.Migration{Version: 4, Direction: source.Up, Identifier: "CREATE 4"})
	sourceStubMigrations.Append(&source.Migration{Version: 4, Direction: source.Down, Identifier: "DROP 4"})
	sourceStubMigrations.Append(&source.Migration{Version: 5, Direction: source.Down, Identifier: "DROP 5"})
	sourceStubMigrations.Append(&source.Migration{Version: 7, Direction: source.Up, Identifier: "CREATE 7"})
	sourceStubMigrations.Append(&source.Migration{Version: 7, Direction: source.Down, Identifier: "DROP 7"})

	// Init the sourceProgrammaticMigrations. Note that the content of the
	// programmatic migrations for version 2 & 7 is defined in the
	// respective unit tests where the sourceProgrammaticMigrations is used.
	sourceProgrammaticMigrations = source.NewMigrations()
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 1, Direction: source.Up, Identifier: "CREATE 1"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 1, Direction: source.Down, Identifier: "DROP 1"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 2, Direction: source.Up, Identifier: ""})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 2, Direction: source.Down, Identifier: ""})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 3, Direction: source.Up, Identifier: "CREATE 3"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 4, Direction: source.Up, Identifier: "CREATE 4"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 4, Direction: source.Down, Identifier: "DROP 4"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 5, Direction: source.Down, Identifier: "DROP 5"})
	sourceProgrammaticMigrations.Append(&source.Migration{Version: 7, Direction: source.Up, Identifier: ""})
}

type DummyInstance struct{ Name string }

func TestNew(t *testing.T) {
	m, err := New("stub://", "stub://")
	if err != nil {
		t.Fatal(err)
	}

	if m.sourceName != srcDrvNameStub {
		t.Errorf("expected stub, got %v", m.sourceName)
	}
	if m.sourceDrv == nil {
		t.Error("expected sourceDrv not to be nil")
	}

	if m.databaseName != dbDrvNameStub {
		t.Errorf("expected stub, got %v", m.databaseName)
	}
	if m.databaseDrv == nil {
		t.Error("expected databaseDrv not to be nil")
	}
}

func ExampleNew() {
	// Read migrations from /home/mattes/migrations and connect to a local postgres database.
	m, err := New("file:///home/mattes/migrations", "postgres://mattes:secret@localhost:5432/database?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	// Migrate all the way up ...
	if err := m.Up(); err != nil && err != ErrNoChange {
		log.Fatal(err)
	}
}

func TestNewWithDatabaseInstance(t *testing.T) {
	dummyDb := &DummyInstance{"database"}
	dbInst, err := dStub.WithInstance(dummyDb, &dStub.Config{})
	if err != nil {
		t.Fatal(err)
	}

	m, err := NewWithDatabaseInstance("stub://", dbDrvNameStub, dbInst)
	if err != nil {
		t.Fatal(err)
	}

	if m.sourceName != srcDrvNameStub {
		t.Errorf("expected stub, got %v", m.sourceName)
	}
	if m.sourceDrv == nil {
		t.Error("expected sourceDrv not to be nil")
	}

	if m.databaseName != dbDrvNameStub {
		t.Errorf("expected stub, got %v", m.databaseName)
	}
	if m.databaseDrv == nil {
		t.Error("expected databaseDrv not to be nil")
	}
}

func ExampleNewWithDatabaseInstance() {
	// Create and use an existing database instance.
	db, err := sql.Open("postgres", "postgres://mattes:secret@localhost:5432/database?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	// Create driver instance from db.
	// Check each driver if it supports the WithInstance function.
	// `import "github.com/golang-migrate/migrate/v4/database/postgres"`
	instance, err := dStub.WithInstance(db, &dStub.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Read migrations from /home/mattes/migrations and connect to a local postgres database.
	m, err := NewWithDatabaseInstance("file:///home/mattes/migrations", "postgres", instance)
	if err != nil {
		log.Fatal(err)
	}

	// Migrate all the way up ...
	if err := m.Up(); err != nil {
		log.Fatal(err)
	}
}

func TestNewWithSourceInstance(t *testing.T) {
	dummySource := &DummyInstance{"source"}
	sInst, err := sStub.WithInstance(dummySource, &sStub.Config{})
	if err != nil {
		t.Fatal(err)
	}

	m, err := NewWithSourceInstance(srcDrvNameStub, sInst, "stub://")
	if err != nil {
		t.Fatal(err)
	}

	if m.sourceName != srcDrvNameStub {
		t.Errorf("expected stub, got %v", m.sourceName)
	}
	if m.sourceDrv == nil {
		t.Error("expected sourceDrv not to be nil")
	}

	if m.databaseName != dbDrvNameStub {
		t.Errorf("expected stub, got %v", m.databaseName)
	}
	if m.databaseDrv == nil {
		t.Error("expected databaseDrv not to be nil")
	}
}

func ExampleNewWithSourceInstance() {
	di := &DummyInstance{"think any client required for a source here"}

	// Create driver instance from DummyInstance di.
	// Check each driver if it support the WithInstance function.
	// `import "github.com/golang-migrate/migrate/v4/source/stub"`
	instance, err := sStub.WithInstance(di, &sStub.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Read migrations from Stub and connect to a local postgres database.
	m, err := NewWithSourceInstance(srcDrvNameStub, instance, "postgres://mattes:secret@localhost:5432/database?sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}

	// Migrate all the way up ...
	if err := m.Up(); err != nil {
		log.Fatal(err)
	}
}

func TestNewWithInstance(t *testing.T) {
	dummyDb := &DummyInstance{"database"}
	dbInst, err := dStub.WithInstance(dummyDb, &dStub.Config{})
	if err != nil {
		t.Fatal(err)
	}

	dummySource := &DummyInstance{"source"}
	sInst, err := sStub.WithInstance(dummySource, &sStub.Config{})
	if err != nil {
		t.Fatal(err)
	}

	m, err := NewWithInstance(srcDrvNameStub, sInst, dbDrvNameStub, dbInst)
	if err != nil {
		t.Fatal(err)
	}

	if m.sourceName != srcDrvNameStub {
		t.Errorf("expected stub, got %v", m.sourceName)
	}
	if m.sourceDrv == nil {
		t.Error("expected sourceDrv not to be nil")
	}

	if m.databaseName != dbDrvNameStub {
		t.Errorf("expected stub, got %v", m.databaseName)
	}
	if m.databaseDrv == nil {
		t.Error("expected databaseDrv not to be nil")
	}
}

func ExampleNewWithInstance() {
	// See NewWithDatabaseInstance and NewWithSourceInstance for an example.
}

func TestClose(t *testing.T) {
	m, _ := New("stub://", "stub://")
	sourceErr, databaseErr := m.Close()
	if sourceErr != nil {
		t.Error(sourceErr)
	}
	if databaseErr != nil {
		t.Error(databaseErr)
	}
}

func TestMigrate(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	tt := []struct {
		version       uint
		expectErr     error
		expectVersion uint
		expectSeq     migrationSequence
	}{
		// migrate all the way Up in single steps
		{
			version:   0,
			expectErr: os.ErrNotExist,
		},
		{
			version:       1,
			expectVersion: 1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
			},
		},
		{
			version:   2,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
			},
		},
		{
			version:       3,
			expectVersion: 3,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
			},
		},
		{
			version:       4,
			expectVersion: 4,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},
		{
			version:       5,
			expectVersion: 5,
			expectSeq: migrationSequence{ // 5 has no up migration
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},
		{
			version:   6,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},
		{
			version:       7,
			expectVersion: 7,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},
		{
			version:   8,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},

		// migrate all the way Down in single steps
		{
			version:   6,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},
		{
			version:       5,
			expectVersion: 5,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
			},
		},
		{
			version:       4,
			expectVersion: 4,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
			},
		},
		{
			version:       3,
			expectVersion: 3,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
		{
			version:   2,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
		{
			version:       1,
			expectVersion: 1,
			expectSeq: migrationSequence{ // 3 has no down migration
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
		{
			version:   0,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},

		// migrate all the way Up in one step
		{
			version:       7,
			expectVersion: 7,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},

		// migrate all the way Down in one step
		{
			version:       1,
			expectVersion: 1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},

		// can't migrate the same version twice
		{
			version:   1,
			expectErr: ErrNoChange,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
	}

	for i, v := range tt {
		err := m.Migrate(v.version)
		if (v.expectErr == os.ErrNotExist && !errors.Is(err, os.ErrNotExist)) ||
			(v.expectErr != os.ErrNotExist && err != v.expectErr) {
			t.Errorf("expected err %v, got %v, in %v", v.expectErr, err, i)

		} else if err == nil {
			version, _, err := m.Version()
			if err != nil {
				t.Error(err)
			}
			if version != v.expectVersion {
				t.Errorf("expected version %v, got %v, in %v", v.expectVersion, version, i)
			}
		}
		equalDbSeq(t, i, v.expectSeq, dbDrv)
	}
}

func TestMigrateDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	err := m.Migrate(1)
	if _, ok := err.(ErrDirty); !ok {
		t.Fatalf("expected ErrDirty, got %v", err)
	}
}

func TestSteps(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	tt := []struct {
		steps         int
		expectErr     error
		expectVersion int
		expectSeq     migrationSequence
	}{
		// step must be != 0
		{
			steps:     0,
			expectErr: ErrNoChange,
		},

		// can't go Down if ErrNilVersion
		{
			steps:     -1,
			expectErr: os.ErrNotExist,
		},

		// migrate all the way Up
		{
			steps:         1,
			expectVersion: 1,
			expectSeq: migrationSequence{
				mr("CREATE 1")},
		},
		{
			steps:         1,
			expectVersion: 3,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
			},
		},
		{
			steps:         1,
			expectVersion: 4,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},
		{
			steps:         1,
			expectVersion: 5,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},
		{
			steps:         1,
			expectVersion: 7,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},
		{
			steps:     1,
			expectErr: os.ErrNotExist,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},

		// migrate all the way Down
		{
			steps:         -1,
			expectVersion: 5,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
			},
		},
		{
			steps:         -1,
			expectVersion: 4,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
			},
		},
		{
			steps:         -1,
			expectVersion: 3,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
		{
			steps:         -1,
			expectVersion: 1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},
		{
			steps:         -1,
			expectVersion: -1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
			},
		},

		// migrate Up in bigger step
		{
			steps:         4,
			expectVersion: 5,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
			},
		},

		// apply one migration, then reaches out of boundary
		{
			steps:         2,
			expectErr:     ErrShortLimit{1},
			expectVersion: 7,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
			},
		},

		// migrate Down in bigger step
		{
			steps:         -4,
			expectVersion: 1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
			},
		},

		// apply one migration, then reaches out of boundary
		{
			steps:         -2,
			expectErr:     ErrShortLimit{1},
			expectVersion: -1,
			expectSeq: migrationSequence{
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
				mr("CREATE 1"),
				mr("CREATE 3"),
				mr("CREATE 4"),
				mr("CREATE 7"),
				mr("DROP 7"),
				mr("DROP 5"),
				mr("DROP 4"),
				mr("DROP 1"),
			},
		},
	}

	for i, v := range tt {
		err := m.Steps(v.steps)
		if (v.expectErr == os.ErrNotExist && !errors.Is(err, os.ErrNotExist)) ||
			(v.expectErr != os.ErrNotExist && err != v.expectErr) {
			t.Errorf("expected err %v, got %v, in %v", v.expectErr, err, i)

		} else if err == nil {
			version, _, err := m.Version()
			if err != ErrNilVersion && err != nil {
				t.Error(err)
			}
			if v.expectVersion == -1 && err != ErrNilVersion {
				t.Errorf("expected ErrNilVersion, got %v, in %v", version, i)

			} else if v.expectVersion >= 0 && version != uint(v.expectVersion) {
				t.Errorf("expected version %v, got %v, in %v", v.expectVersion, version, i)
			}
		}
		equalDbSeq(t, i, v.expectSeq, dbDrv)
	}
}

func TestStepsDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	err := m.Steps(1)
	if _, ok := err.(ErrDirty); !ok {
		t.Fatalf("expected ErrDirty, got %v", err)
	}
}

func TestUpAndDown(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	// go Up first
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	expectedSequence := migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	// go Down
	if err := m.Down(); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 1, expectedSequence, dbDrv)

	// go 1 Up and then all the way Up
	if err := m.Steps(1); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
		mr("CREATE 1"),
	}
	equalDbSeq(t, 2, expectedSequence, dbDrv)

	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
	}
	equalDbSeq(t, 3, expectedSequence, dbDrv)

	// go 1 Down and then all the way Down
	if err := m.Steps(-1); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
	}
	equalDbSeq(t, 1, expectedSequence, dbDrv)

	if err := m.Down(); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CREATE 7"),
		mr("DROP 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 1, expectedSequence, dbDrv)
}

func TestProgrammaticMigration(t *testing.T) {
	m, _ := New("stub://", "stub://", WithProgrammaticMigrations(
		map[uint]ProgrammaticMigrEntry{
			2: {
				ResetVersionOnError: true,
				ProgrammaticMigr: func(m *Migration,
					driver database.Driver) error {

					return driver.Run(
						strings.NewReader("CALLBACK 2"),
					)
				},
			},
			7: {
				ResetVersionOnError: true,
				ProgrammaticMigr: func(m *Migration,
					driver database.Driver) error {

					return driver.Run(
						strings.NewReader("CALLBACK 7"),
					)
				},
			},
		},
	))
	m.sourceDrv.(*sStub.Stub).Migrations = sourceProgrammaticMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	// go Up first
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	expectedSequence := migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 7")) {
		t.Fatalf("expected database last migration to be callback 7, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// go Down
	if err := m.Down(); err != nil {
		t.Fatal(err)
	}

	// Note that "CALLBACK 7" isn't repeated when going down, as that
	// migration source only contains an .up migration for version 7, and
	// not a .down migration.
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("CALLBACK 2"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 1, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("DROP 1")) {
		t.Fatalf("expected database last migration to be DROP 1, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// go 1 Up and then all the way Up
	if err := m.Steps(1); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("CALLBACK 2"),
		mr("DROP 1"),
		mr("CREATE 1"),
	}
	equalDbSeq(t, 2, expectedSequence, dbDrv)

	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("CALLBACK 2"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CALLBACK 2"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
	}
	equalDbSeq(t, 3, expectedSequence, dbDrv)
}

// TestProgrammaticMigrationError simulates a programmatic migration that fails,
// and ensures that:
//  1. The migration process stops and returns the programmatic migration error.
//  2. The migration version is set to the migration version set prior to
//     executing the programmatic migration if it errors.
//  3. Re-running Up will re-attempt the programmatic migration.
//  4. Down will not re-execute a programmatic migration if it errored, as the
//     version should have been set to the version prior to executing the
//     programmatic migration.
//  5. Once the programmatic migration succeeds, the migration can finalize and
//     reach the latest version cleanly.
//  6. Down will only execute the programmatic migration at a given version if a
//     ".down" file is defined for the programmatic migration.
func TestProgrammaticMigrationError(t *testing.T) {
	var (
		cbError    = errors.New("programmatic migration failure")
		shouldFail = true
	)

	// The programmatic migration for version 2 will error if
	// shouldFail == false, and succeed if it is set to true.
	m, _ := New("stub://", "stub://", WithProgrammaticMigrations(
		map[uint]ProgrammaticMigrEntry{
			2: {
				ResetVersionOnError: true,
				ProgrammaticMigr: func(migr *Migration,
					driver database.Driver) error {

					if shouldFail {
						err := driver.Run(
							strings.NewReader(
								"CALLBACK 2 FAILURE",
							))
						if err != nil {
							return err
						}

						return cbError
					}

					err := driver.Run(strings.NewReader(
						"CALLBACK 2 SUCCESS",
					))
					if err != nil {
						return err
					}

					return nil
				},
			},
			7: {
				ResetVersionOnError: true,
				ProgrammaticMigr: func(m *Migration,
					driver database.Driver) error {

					return driver.Run(
						strings.NewReader("CALLBACK 7"),
					)
				},
			},
		},
	))

	m.sourceDrv.(*sStub.Stub).Migrations = sourceProgrammaticMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	// 1) Run Up — the programmatic migration for 2 should fail.
	err := m.Up()
	if !errors.Is(err, cbError) {
		t.Fatal("expected cbError from failing programmatic migration")
	}

	// The sequence should show the migrations prior to version 2, and then
	// the failing callback for version 2.
	expectedSequence := migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 2 FAILURE")) {
		t.Fatalf("expected database last migration to be callback 2, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// 2) Due to that the programmatic migration errored, the version should
	// have been reset to the version set before the programmatic migration
	// was executed.
	checkVersion(t, dbDrv, 1)

	// 3) Re-run Up — since the database version is set to version 1, it
	// should try the programmatic migration again at version 2 and fail
	// again.
	err = m.Up()
	if !errors.Is(err, cbError) {
		t.Fatal("expected cbError from failing programmatic migration")
	}

	// The programmatic migration for version 2 should now have failed twice.
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 2 FAILURE")) {
		t.Fatalf("expected database last migration to be callback 2, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// The version should have been reset to 1 once again.
	checkVersion(t, dbDrv, 1)

	// 4) Execute down — as the version should have been reset to 1, this
	// should not re-execute the programmatic migration for version 2.
	err = m.Down()
	if err != nil {
		t.Fatalf("unexpected Down error: %v", err)
	}

	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("DROP 1")) {
		t.Fatalf("expected database last migration to be DROP 1, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// The version should now be at -1, as the migration for version 1 has
	// been dropped.
	checkVersion(t, dbDrv, -1)

	// 5) Make the callback succeed by setting shouldFail to false and run
	// again. It should now successfully re-run the callback and then
	// finalize the migration cleanly to the latest version (7).
	shouldFail = false

	err = m.Up()
	if err != nil {
		t.Fatalf("unexpected Up error: %v", err)
	}

	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CALLBACK 2 SUCCESS"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	// And the last run migration should be from the callback.
	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 7")) {
		t.Fatalf("expected last run migration to be from the "+
			"programmatic migration, got %q",
			dbDrv.LastRunMigration)
	}

	// The version should now be the latest migration version 7 and not
	// dirty.
	checkVersion(t, dbDrv, 7)

	// Try Up again — it should be a no-op since we are at the latest
	// version, and shouldn't run the programmatic migration again.
	err = m.Up()
	if !errors.Is(err, ErrNoChange) {
		t.Fatalf("unexpected error after programmatic migration "+
			"succeed: %v", err)
	}

	// Ensure the callback wasn't re-run.
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CALLBACK 2 SUCCESS"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	// And the last run migration should be from the previous callback.
	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 7")) {
		t.Fatalf("expected last run migration to be from programmatic"+
			"migration, got %q", dbDrv.LastRunMigration)
	}

	checkVersion(t, dbDrv, 7)

	// 6) Finally, running Down now should only execute the programmatic
	// migration at for version 2, as that programmatic migration is the
	// only programmatic migration that has a defined ".down" file.
	// We start setting the shouldFail boolean to true again, be able to
	// test the retry logic for .down migrations for programmatic migrations
	// as well.
	shouldFail = true

	err = m.Down()
	if !errors.Is(err, cbError) {
		t.Fatal("expected cbError from failing programmatic migration")
	}

	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CALLBACK 2 SUCCESS"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("CALLBACK 2 FAILURE"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 2 FAILURE")) {
		t.Fatalf("expected database last migration to be callback 2, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// The version should be at 2, as the programmatic migration for that
	// version should have failed.
	checkVersion(t, dbDrv, 2)

	// Finally, we set shouldFail to false again, and retry the downgrade.
	// This should now succeed and execute the programmatic migration for
	// version 2 successfully.
	shouldFail = false

	err = m.Down()
	if err != nil {
		t.Fatalf("unexpected down error: %v", err)
	}

	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 FAILURE"),
		mr("DROP 1"),
		mr("CREATE 1"),
		mr("CALLBACK 2 SUCCESS"),
		mr("CREATE 3"),
		mr("CREATE 4"),
		mr("CALLBACK 7"),
		mr("DROP 5"),
		mr("DROP 4"),
		mr("CALLBACK 2 FAILURE"),
		mr("CALLBACK 2 SUCCESS"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv)

	if !bytes.Equal(dbDrv.LastRunMigration, []byte("DROP 1")) {
		t.Fatalf("expected database last migration to be DROP 1, "+
			"got %s", dbDrv.LastRunMigration)
	}

	// The version should now be at -1, as the migration for version 1 has
	// been dropped.
	checkVersion(t, dbDrv, -1)
}

// TestProgrammaticMigrationErrorNoRerun ensures that when ResetVersionOnError
// is false, a failing programmatic migration leaves the database dirty at the
// current migration version instead of rewinding to the prior version.
func TestProgrammaticMigrationErrorNoRerun(t *testing.T) {
	cbError := errors.New("programmatic migration failure")

	m, _ := New("stub://", "stub://", WithProgrammaticMigrations(
		map[uint]ProgrammaticMigrEntry{
			2: {
				ResetVersionOnError: false,
				ProgrammaticMigr: func(migr *Migration,
					driver database.Driver) error {

					if err := driver.Run(strings.NewReader(
						"CALLBACK 2 FAILURE",
					)); err != nil {
						return err
					}
					return cbError
				},
			},
		},
	))

	m.sourceDrv.(*sStub.Stub).Migrations = sourceProgrammaticMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	err := m.Up()
	if !errors.Is(err, cbError) {
		t.Fatalf("expected cbError from failing programmatic "+
			"migration, got %v", err)
	}

	version, dirty, err := dbDrv.Version()
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if version != 2 || !dirty {
		t.Fatalf("expected version 2 dirty after failure, got "+
			"version=%d dirty=%v", version, dirty)
	}
	if !bytes.Equal(dbDrv.LastRunMigration, []byte("CALLBACK 2 FAILURE")) {
		t.Fatalf("expected last migration body to be failure "+
			"callback, got %s", dbDrv.LastRunMigration)
	}

	// Re-running Up again, should fail with ErrDirty for version 2, as
	// the database should be in a dirty state.
	if err = m.Up(); !errors.Is(err, ErrDirty{2}) {
		t.Fatalf("expected version 2 dirty error on re-running Up, "+
			"got %v", err)
	}
}

// TestMigrationTypeCombos test ensures that a migration cannot be both an SQL
// migration and a programmatic migration at the same time. It also tests that a
// migration can be neither an SQL migration nor a programmatic migration.
func TestMigrationTypeCombos(t *testing.T) {
	// Test that a migration can't be both an SQL migration and a
	// programmatic migration at the same time.
	m1, _ := New("stub://", "stub://", WithProgrammaticMigrations(
		map[uint]ProgrammaticMigrEntry{
			2: {
				ResetVersionOnError: true,
				ProgrammaticMigr: func(m *Migration,
					driver database.Driver) error {

					return driver.Run(
						strings.NewReader("CALLBACK 2"),
					)
				},
			},
		},
	))
	dbDrv1 := m1.databaseDrv.(*dStub.Stub)

	bothTypeMig := source.NewMigrations()
	bothTypeMig.Append(
		&source.Migration{Version: 1, Direction: source.Up,
			Identifier: "CREATE 1"},
	)
	bothTypeMig.Append(
		&source.Migration{Version: 1, Direction: source.Down,
			Identifier: "DROP 1"},
	)
	bothTypeMig.Append(
		&source.Migration{Version: 2, Direction: source.Up,
			Identifier: "CREATE 2"},
	)
	bothTypeMig.Append(
		&source.Migration{Version: 2, Direction: source.Down,
			Identifier: "DROP 2"},
	)

	m1.sourceDrv.(*sStub.Stub).Migrations = bothTypeMig

	err := m1.Up()
	if err == nil || !errors.Is(err, ErrMixedMigration) {
		t.Fatalf("expected %v, but got %v", ErrMixedMigration, err)
	}

	// We expect that only migration 1 should have been executed, as the
	// migration for version 2 should have errored before being executed.
	expectedSequence := migrationSequence{
		mr("CREATE 1"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv1)

	// The db version should only be at 1, as the migration for version 2
	// should have errored before any version was set.
	checkVersion(t, dbDrv1, 1)

	err = m1.Down()
	if err != nil {
		t.Fatalf("expected %v, but got %v", ErrMixedMigration, err)
	}

	// As the up migration for version 2 was never run, we expect that we
	// only execute the down migration for version 1 as well.
	expectedSequence = migrationSequence{
		mr("CREATE 1"),
		mr("DROP 1"),
	}
	equalDbSeq(t, 0, expectedSequence, dbDrv1)

	checkVersion(t, dbDrv1, -1)

	// Test that a migration can be neither an SQL migration nor a
	// programmatic migration.
	m2, _ := New("stub://", "stub://")
	dbDrv2 := m2.databaseDrv.(*dStub.Stub)

	noTypeMig := source.NewMigrations()
	noTypeMig.Append(
		&source.Migration{Version: 1, Direction: source.Up,
			Identifier: ""},
	)
	noTypeMig.Append(
		&source.Migration{Version: 1, Direction: source.Down,
			Identifier: ""},
	)

	m2.sourceDrv.(*sStub.Stub).Migrations = noTypeMig

	err = m2.Up()
	if err != nil {
		t.Fatal("unexpected error when a migration was neither an " +
			"SQL migration nor a programmatic migration")
	}

	// We expect that no migration sequence has been executed, as the
	// migration for version 1 didn't contain any SQL or programmatic
	// migration.
	equalDbSeq(t, 0, migrationSequence{}, dbDrv2)

	// The version should have been changed to 1 though, as the migration
	// should be seen as a valid migration.
	checkVersion(t, dbDrv2, 1)

	// The same behavior should be observed when downgrading.
	err = m2.Down()
	if err != nil {
		t.Fatal("unexpected error when a migration was neither an " +
			"SQL migration nor a programmatic migration")
	}

	// We still expect no migration sequence.
	equalDbSeq(t, 0, migrationSequence{}, dbDrv2)

	// The version should have been changed to -1 though, as the migration
	// should be seen as a valid migration.
	checkVersion(t, dbDrv2, -1)
}

func TestUpDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	err := m.Up()
	if _, ok := err.(ErrDirty); !ok {
		t.Fatalf("expected ErrDirty, got %v", err)
	}
}

func TestDownDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	err := m.Down()
	if _, ok := err.(ErrDirty); !ok {
		t.Fatalf("expected ErrDirty, got %v", err)
	}
}

func TestDrop(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations
	dbDrv := m.databaseDrv.(*dStub.Stub)

	if err := m.Drop(); err != nil {
		t.Fatal(err)
	}

	if dbDrv.MigrationSequence[len(dbDrv.MigrationSequence)-1] != dStub.DROP {
		t.Fatalf("expected database to DROP, got sequence %v", dbDrv.MigrationSequence)
	}
}

func TestVersion(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)

	_, _, err := m.Version()
	if err != ErrNilVersion {
		t.Fatalf("expected ErrNilVersion, got %v", err)
	}

	if err := dbDrv.Run(bytes.NewBufferString("1_up")); err != nil {
		t.Fatal(err)
	}

	if err := dbDrv.SetVersion(1, false); err != nil {
		t.Fatal(err)
	}

	v, _, err := m.Version()
	if err != nil {
		t.Fatal(err)
	}

	if v != 1 {
		t.Fatalf("expected version 1, got %v", v)
	}
}

func TestRun(t *testing.T) {
	m, _ := New("stub://", "stub://")

	mx, err := NewMigration(nil, "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}

	if err := m.Run(mx); err != nil {
		t.Fatal(err)
	}

	v, _, err := m.Version()
	if err != nil {
		t.Fatal(err)
	}

	if v != 2 {
		t.Errorf("expected version 2, got %v", v)
	}
}

func TestRunDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	migr, err := NewMigration(nil, "", 1, 2)
	if err != nil {
		t.Fatal(err)
	}

	err = m.Run(migr)
	if _, ok := err.(ErrDirty); !ok {
		t.Fatalf("expected ErrDirty, got %v", err)
	}
}

func TestForce(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations

	if err := m.Force(7); err != nil {
		t.Fatal(err)
	}

	v, dirty, err := m.Version()
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Errorf("expected dirty to be false")
	}
	if v != 7 {
		t.Errorf("expected version to be 7")
	}
}

func TestForceDirty(t *testing.T) {
	m, _ := New("stub://", "stub://")
	dbDrv := m.databaseDrv.(*dStub.Stub)
	if err := dbDrv.SetVersion(0, true); err != nil {
		t.Fatal(err)
	}

	if err := m.Force(1); err != nil {
		t.Fatal(err)
	}
}

func TestRead(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations

	tt := []struct {
		from             int
		to               int
		expectErr        error
		expectMigrations migrationSequence
	}{
		{from: -1, to: -1, expectErr: ErrNoChange},
		{from: -1, to: 0, expectErr: os.ErrNotExist},
		{from: -1, to: 1, expectErr: nil, expectMigrations: newMigSeq(M(1))},
		{from: -1, to: 2, expectErr: os.ErrNotExist},
		{from: -1, to: 3, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3))},
		{from: -1, to: 4, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3), M(4))},
		{from: -1, to: 5, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3), M(4), M(5))},
		{from: -1, to: 6, expectErr: os.ErrNotExist},
		{from: -1, to: 7, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3), M(4), M(5), M(7))},
		{from: -1, to: 8, expectErr: os.ErrNotExist},

		{from: 0, to: -1, expectErr: os.ErrNotExist},
		{from: 0, to: 0, expectErr: os.ErrNotExist},
		{from: 0, to: 1, expectErr: os.ErrNotExist},
		{from: 0, to: 2, expectErr: os.ErrNotExist},
		{from: 0, to: 3, expectErr: os.ErrNotExist},
		{from: 0, to: 4, expectErr: os.ErrNotExist},
		{from: 0, to: 5, expectErr: os.ErrNotExist},
		{from: 0, to: 6, expectErr: os.ErrNotExist},
		{from: 0, to: 7, expectErr: os.ErrNotExist},
		{from: 0, to: 8, expectErr: os.ErrNotExist},

		{from: 1, to: -1, expectErr: nil, expectMigrations: newMigSeq(M(1, -1))},
		{from: 1, to: 0, expectErr: os.ErrNotExist},
		{from: 1, to: 1, expectErr: ErrNoChange},
		{from: 1, to: 2, expectErr: os.ErrNotExist},
		{from: 1, to: 3, expectErr: nil, expectMigrations: newMigSeq(M(3))},
		{from: 1, to: 4, expectErr: nil, expectMigrations: newMigSeq(M(3), M(4))},
		{from: 1, to: 5, expectErr: nil, expectMigrations: newMigSeq(M(3), M(4), M(5))},
		{from: 1, to: 6, expectErr: os.ErrNotExist},
		{from: 1, to: 7, expectErr: nil, expectMigrations: newMigSeq(M(3), M(4), M(5), M(7))},
		{from: 1, to: 8, expectErr: os.ErrNotExist},

		{from: 2, to: -1, expectErr: os.ErrNotExist},
		{from: 2, to: 0, expectErr: os.ErrNotExist},
		{from: 2, to: 1, expectErr: os.ErrNotExist},
		{from: 2, to: 2, expectErr: os.ErrNotExist},
		{from: 2, to: 3, expectErr: os.ErrNotExist},
		{from: 2, to: 4, expectErr: os.ErrNotExist},
		{from: 2, to: 5, expectErr: os.ErrNotExist},
		{from: 2, to: 6, expectErr: os.ErrNotExist},
		{from: 2, to: 7, expectErr: os.ErrNotExist},
		{from: 2, to: 8, expectErr: os.ErrNotExist},

		{from: 3, to: -1, expectErr: nil, expectMigrations: newMigSeq(M(3, 1), M(1, -1))},
		{from: 3, to: 0, expectErr: os.ErrNotExist},
		{from: 3, to: 1, expectErr: nil, expectMigrations: newMigSeq(M(3, 1))},
		{from: 3, to: 2, expectErr: os.ErrNotExist},
		{from: 3, to: 3, expectErr: ErrNoChange},
		{from: 3, to: 4, expectErr: nil, expectMigrations: newMigSeq(M(4))},
		{from: 3, to: 5, expectErr: nil, expectMigrations: newMigSeq(M(4), M(5))},
		{from: 3, to: 6, expectErr: os.ErrNotExist},
		{from: 3, to: 7, expectErr: nil, expectMigrations: newMigSeq(M(4), M(5), M(7))},
		{from: 3, to: 8, expectErr: os.ErrNotExist},

		{from: 4, to: -1, expectErr: nil, expectMigrations: newMigSeq(M(4, 3), M(3, 1), M(1, -1))},
		{from: 4, to: 0, expectErr: os.ErrNotExist},
		{from: 4, to: 1, expectErr: nil, expectMigrations: newMigSeq(M(4, 3), M(3, 1))},
		{from: 4, to: 2, expectErr: os.ErrNotExist},
		{from: 4, to: 3, expectErr: nil, expectMigrations: newMigSeq(M(4, 3))},
		{from: 4, to: 4, expectErr: ErrNoChange},
		{from: 4, to: 5, expectErr: nil, expectMigrations: newMigSeq(M(5))},
		{from: 4, to: 6, expectErr: os.ErrNotExist},
		{from: 4, to: 7, expectErr: nil, expectMigrations: newMigSeq(M(5), M(7))},
		{from: 4, to: 8, expectErr: os.ErrNotExist},

		{from: 5, to: -1, expectErr: nil, expectMigrations: newMigSeq(M(5, 4), M(4, 3), M(3, 1), M(1, -1))},
		{from: 5, to: 0, expectErr: os.ErrNotExist},
		{from: 5, to: 1, expectErr: nil, expectMigrations: newMigSeq(M(5, 4), M(4, 3), M(3, 1))},
		{from: 5, to: 2, expectErr: os.ErrNotExist},
		{from: 5, to: 3, expectErr: nil, expectMigrations: newMigSeq(M(5, 4), M(4, 3))},
		{from: 5, to: 4, expectErr: nil, expectMigrations: newMigSeq(M(5, 4))},
		{from: 5, to: 5, expectErr: ErrNoChange},
		{from: 5, to: 6, expectErr: os.ErrNotExist},
		{from: 5, to: 7, expectErr: nil, expectMigrations: newMigSeq(M(7))},
		{from: 5, to: 8, expectErr: os.ErrNotExist},

		{from: 6, to: -1, expectErr: os.ErrNotExist},
		{from: 6, to: 0, expectErr: os.ErrNotExist},
		{from: 6, to: 1, expectErr: os.ErrNotExist},
		{from: 6, to: 2, expectErr: os.ErrNotExist},
		{from: 6, to: 3, expectErr: os.ErrNotExist},
		{from: 6, to: 4, expectErr: os.ErrNotExist},
		{from: 6, to: 5, expectErr: os.ErrNotExist},
		{from: 6, to: 6, expectErr: os.ErrNotExist},
		{from: 6, to: 7, expectErr: os.ErrNotExist},
		{from: 6, to: 8, expectErr: os.ErrNotExist},

		{from: 7, to: -1, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4), M(4, 3), M(3, 1), M(1, -1))},
		{from: 7, to: 0, expectErr: os.ErrNotExist},
		{from: 7, to: 1, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4), M(4, 3), M(3, 1))},
		{from: 7, to: 2, expectErr: os.ErrNotExist},
		{from: 7, to: 3, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4), M(4, 3))},
		{from: 7, to: 4, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4))},
		{from: 7, to: 5, expectErr: nil, expectMigrations: newMigSeq(M(7, 5))},
		{from: 7, to: 6, expectErr: os.ErrNotExist},
		{from: 7, to: 7, expectErr: ErrNoChange},
		{from: 7, to: 8, expectErr: os.ErrNotExist},

		{from: 8, to: -1, expectErr: os.ErrNotExist},
		{from: 8, to: 0, expectErr: os.ErrNotExist},
		{from: 8, to: 1, expectErr: os.ErrNotExist},
		{from: 8, to: 2, expectErr: os.ErrNotExist},
		{from: 8, to: 3, expectErr: os.ErrNotExist},
		{from: 8, to: 4, expectErr: os.ErrNotExist},
		{from: 8, to: 5, expectErr: os.ErrNotExist},
		{from: 8, to: 6, expectErr: os.ErrNotExist},
		{from: 8, to: 7, expectErr: os.ErrNotExist},
		{from: 8, to: 8, expectErr: os.ErrNotExist},
	}

	for i, v := range tt {
		ret := make(chan interface{})
		go m.read(v.from, v.to, ret)
		migrations, err := migrationsFromChannel(ret)

		if (v.expectErr == os.ErrNotExist && !errors.Is(err, os.ErrNotExist)) ||
			(v.expectErr != os.ErrNotExist && v.expectErr != err) {
			t.Errorf("expected %v, got %v, in %v", v.expectErr, err, i)
			t.Logf("%v, in %v", migrations, i)
		}
		if len(v.expectMigrations) > 0 {
			equalMigSeq(t, i, v.expectMigrations, migrations)
		}
	}
}

func TestReadUp(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations

	tt := []struct {
		from             int
		limit            int // -1 means no limit
		expectErr        error
		expectMigrations migrationSequence
	}{
		{from: -1, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3), M(4), M(5), M(7))},
		{from: -1, limit: 0, expectErr: ErrNoChange},
		{from: -1, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(1))},
		{from: -1, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(1), M(3))},

		{from: 0, limit: -1, expectErr: os.ErrNotExist},
		{from: 0, limit: 0, expectErr: os.ErrNotExist},
		{from: 0, limit: 1, expectErr: os.ErrNotExist},
		{from: 0, limit: 2, expectErr: os.ErrNotExist},

		{from: 1, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(3), M(4), M(5), M(7))},
		{from: 1, limit: 0, expectErr: ErrNoChange},
		{from: 1, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(3))},
		{from: 1, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(3), M(4))},

		{from: 2, limit: -1, expectErr: os.ErrNotExist},
		{from: 2, limit: 0, expectErr: os.ErrNotExist},
		{from: 2, limit: 1, expectErr: os.ErrNotExist},
		{from: 2, limit: 2, expectErr: os.ErrNotExist},

		{from: 3, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(4), M(5), M(7))},
		{from: 3, limit: 0, expectErr: ErrNoChange},
		{from: 3, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(4))},
		{from: 3, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(4), M(5))},

		{from: 4, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(5), M(7))},
		{from: 4, limit: 0, expectErr: ErrNoChange},
		{from: 4, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(5))},
		{from: 4, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(5), M(7))},

		{from: 5, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(7))},
		{from: 5, limit: 0, expectErr: ErrNoChange},
		{from: 5, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(7))},
		{from: 5, limit: 2, expectErr: ErrShortLimit{1}, expectMigrations: newMigSeq(M(7))},

		{from: 6, limit: -1, expectErr: os.ErrNotExist},
		{from: 6, limit: 0, expectErr: os.ErrNotExist},
		{from: 6, limit: 1, expectErr: os.ErrNotExist},
		{from: 6, limit: 2, expectErr: os.ErrNotExist},

		{from: 7, limit: -1, expectErr: ErrNoChange},
		{from: 7, limit: 0, expectErr: ErrNoChange},
		{from: 7, limit: 1, expectErr: os.ErrNotExist},
		{from: 7, limit: 2, expectErr: os.ErrNotExist},

		{from: 8, limit: -1, expectErr: os.ErrNotExist},
		{from: 8, limit: 0, expectErr: os.ErrNotExist},
		{from: 8, limit: 1, expectErr: os.ErrNotExist},
		{from: 8, limit: 2, expectErr: os.ErrNotExist},
	}

	for i, v := range tt {
		ret := make(chan interface{})
		go m.readUp(v.from, v.limit, ret)
		migrations, err := migrationsFromChannel(ret)

		if (v.expectErr == os.ErrNotExist && !errors.Is(err, os.ErrNotExist)) ||
			(v.expectErr != os.ErrNotExist && v.expectErr != err) {
			t.Errorf("expected %v, got %v, in %v", v.expectErr, err, i)
			t.Logf("%v, in %v", migrations, i)
		}
		if len(v.expectMigrations) > 0 {
			equalMigSeq(t, i, v.expectMigrations, migrations)
		}
	}
}

func TestReadDown(t *testing.T) {
	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations

	tt := []struct {
		from             int
		limit            int // -1 means no limit
		expectErr        error
		expectMigrations migrationSequence
	}{
		{from: -1, limit: -1, expectErr: ErrNoChange},
		{from: -1, limit: 0, expectErr: ErrNoChange},
		{from: -1, limit: 1, expectErr: os.ErrNotExist},
		{from: -1, limit: 2, expectErr: os.ErrNotExist},

		{from: 0, limit: -1, expectErr: os.ErrNotExist},
		{from: 0, limit: 0, expectErr: os.ErrNotExist},
		{from: 0, limit: 1, expectErr: os.ErrNotExist},
		{from: 0, limit: 2, expectErr: os.ErrNotExist},

		{from: 1, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(1, -1))},
		{from: 1, limit: 0, expectErr: ErrNoChange},
		{from: 1, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(1, -1))},
		{from: 1, limit: 2, expectErr: ErrShortLimit{1}, expectMigrations: newMigSeq(M(1, -1))},

		{from: 2, limit: -1, expectErr: os.ErrNotExist},
		{from: 2, limit: 0, expectErr: os.ErrNotExist},
		{from: 2, limit: 1, expectErr: os.ErrNotExist},
		{from: 2, limit: 2, expectErr: os.ErrNotExist},

		{from: 3, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(3, 1), M(1, -1))},
		{from: 3, limit: 0, expectErr: ErrNoChange},
		{from: 3, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(3, 1))},
		{from: 3, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(3, 1), M(1, -1))},

		{from: 4, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(4, 3), M(3, 1), M(1, -1))},
		{from: 4, limit: 0, expectErr: ErrNoChange},
		{from: 4, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(4, 3))},
		{from: 4, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(4, 3), M(3, 1))},

		{from: 5, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(5, 4), M(4, 3), M(3, 1), M(1, -1))},
		{from: 5, limit: 0, expectErr: ErrNoChange},
		{from: 5, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(5, 4))},
		{from: 5, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(5, 4), M(4, 3))},

		{from: 6, limit: -1, expectErr: os.ErrNotExist},
		{from: 6, limit: 0, expectErr: os.ErrNotExist},
		{from: 6, limit: 1, expectErr: os.ErrNotExist},
		{from: 6, limit: 2, expectErr: os.ErrNotExist},

		{from: 7, limit: -1, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4), M(4, 3), M(3, 1), M(1, -1))},
		{from: 7, limit: 0, expectErr: ErrNoChange},
		{from: 7, limit: 1, expectErr: nil, expectMigrations: newMigSeq(M(7, 5))},
		{from: 7, limit: 2, expectErr: nil, expectMigrations: newMigSeq(M(7, 5), M(5, 4))},

		{from: 8, limit: -1, expectErr: os.ErrNotExist},
		{from: 8, limit: 0, expectErr: os.ErrNotExist},
		{from: 8, limit: 1, expectErr: os.ErrNotExist},
		{from: 8, limit: 2, expectErr: os.ErrNotExist},
	}

	for i, v := range tt {
		ret := make(chan interface{})
		go m.readDown(v.from, v.limit, ret)
		migrations, err := migrationsFromChannel(ret)

		if (v.expectErr == os.ErrNotExist && !errors.Is(err, os.ErrNotExist)) ||
			(v.expectErr != os.ErrNotExist && v.expectErr != err) {
			t.Errorf("expected %v, got %v, in %v", v.expectErr, err, i)
			t.Logf("%v, in %v", migrations, i)
		}
		if len(v.expectMigrations) > 0 {
			equalMigSeq(t, i, v.expectMigrations, migrations)
		}
	}
}

func TestLock(t *testing.T) {
	m, _ := New("stub://", "stub://")
	if err := m.lock(); err != nil {
		t.Fatal(err)
	}

	if err := m.lock(); err == nil {
		t.Fatal("should be locked already")
	}
}

func migrationsFromChannel(ret chan interface{}) ([]*Migration, error) {
	slice := make([]*Migration, 0)
	for r := range ret {
		switch t := r.(type) {
		case error:
			return slice, t

		case *Migration:
			slice = append(slice, t)
		}
	}
	return slice, nil
}

type migrationSequence []*Migration

func newMigSeq(migr ...*Migration) migrationSequence {
	return migr
}

func (m *migrationSequence) add(migr ...*Migration) migrationSequence { // nolint:unused
	*m = append(*m, migr...)
	return *m
}

func (m *migrationSequence) bodySequence() []string {
	r := make([]string, 0)
	for _, v := range *m {
		if v.Body != nil {
			body, err := io.ReadAll(v.Body)
			if err != nil {
				panic(err) // that should never happen
			}

			// reset body reader
			// TODO: is there a better/nicer way?
			v.Body = io.NopCloser(bytes.NewReader(body))

			r = append(r, string(body[:]))
		} else {
			r = append(r, "<empty>")
		}
	}
	return r
}

// M is a convenience func to create a new *Migration
func M(version uint, targetVersion ...int) *Migration {
	if len(targetVersion) > 1 {
		panic("only one targetVersion allowed")
	}
	ts := int(version)
	if len(targetVersion) == 1 {
		ts = targetVersion[0]
	}

	m, _ := New("stub://", "stub://")
	m.sourceDrv.(*sStub.Stub).Migrations = sourceStubMigrations
	migr, err := m.newMigration(version, ts)
	if err != nil {
		panic(err)
	}
	return migr
}

// mr is a convenience func to create a new *Migration from the raw database query
func mr(value string) *Migration {
	return &Migration{
		Body: io.NopCloser(strings.NewReader(value)),
	}
}

func equalMigSeq(t *testing.T, i int, expected, got migrationSequence) {
	if len(expected) != len(got) {
		t.Errorf("expected migrations %v, got %v, in %v", expected, got, i)

	} else {
		for ii := 0; ii < len(expected); ii++ {
			if expected[ii].Version != got[ii].Version {
				t.Errorf("expected version %v, got %v, in %v", expected[ii].Version, got[ii].Version, i)
			}

			if expected[ii].TargetVersion != got[ii].TargetVersion {
				t.Errorf("expected targetVersion %v, got %v, in %v", expected[ii].TargetVersion, got[ii].TargetVersion, i)
			}
		}
	}
}

func equalDbSeq(t *testing.T, i int, expected migrationSequence, got *dStub.Stub) {
	bs := expected.bodySequence()
	if !got.EqualSequence(bs) {
		t.Fatalf("\nexpected sequence %v,\ngot               %v, in %v", bs, got.MigrationSequence, i)
	}
}

// checkVersion is a helper function to check the migration version and dirty
// state.
func checkVersion(t *testing.T, dbDrv *dStub.Stub, expVer int) {
	v, dirty, err := dbDrv.Version()
	if err != nil {
		t.Fatal(err)
	}

	if v != expVer {
		t.Fatalf("expected version %d, got v=%d", expVer, v)
	}
	if dirty {
		t.Fatalf("expected clean version, but was dirty")
	}
}
