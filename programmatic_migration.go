package migrate

import "github.com/golang-migrate/migrate/v4/database"

// ProgrammaticMigration is a Golang function type that can be used to execute
// a Golang-based migration. The Golang function receives the migration and
// the database driver as arguments.
type ProgrammaticMigration func(migr *Migration, driver database.Driver) error

// options is a set of optional parameters that can be set when creating a
// Migrate instance.
type options struct {
	// programmaticMigrations is a map of ProgrammaticMigration functions
	// that can be used to execute a Golang-based migration. The key is the
	// migration version, and the value is the Golang function that should
	// be run.
	programmaticMigrations map[uint]ProgrammaticMigration
}

// defaultOptions returns a new options struct with default values.
func defaultOptions() options {
	return options{
		programmaticMigrations: make(map[uint]ProgrammaticMigration),
	}
}

// Option is a function that can be used to set options on a Migrate instance.
type Option func(*options)

// WithProgrammaticMigrations is an option that can be used to set a map of
// ProgrammaticMigration functions that can be used to execute a Golang-based
// migration step. The key is the migration version, and the value is the
// Golang function that should be run. An error returned from the programmatic
// migration will cause the migration to fail but will **not** set the database
// to a dirty state. Additionally, if the programmatic migration fails, the
// database version will be reset to the last set version before executing the
// programmatic migration. The effect of this is that the programmatic migration
// will be re-run on the next startup of the database.
func WithProgrammaticMigrations(pMigrs map[uint]ProgrammaticMigration) Option {
	return func(o *options) {
		for version, pMigr := range pMigrs {
			WithProgrammaticMigration(version, pMigr)(o)
		}
	}
}

// WithProgrammaticMigration is an option that can be used to set a
// ProgrammaticMigration function that can be used to execute a Golang based
// migration.  An error returned from the programmatic migration will cause the
// migration to fail but will **not** set the database to a dirty state.
// Additionally, if the programmatic migration fails, the database version will
// be reset to the last set version before executing the programmatic migration.
// The effect of this is that the programmatic migration will be re-run on the
// next startup of the database.
func WithProgrammaticMigration(version uint,
	pMigr ProgrammaticMigration) Option {

	return func(o *options) {
		o.programmaticMigrations[version] = pMigr
	}
}
