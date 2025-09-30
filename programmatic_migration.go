package migrate

import "github.com/golang-migrate/migrate/v4/database"

// ProgrammaticMigration is a callback function type that can be used to execute
// a Golang based migration step after a SQL based migration step has been
// executed. The callback function receives the migration and the database
// driver as arguments.
type ProgrammaticMigration func(migr *Migration, driver database.Driver) error

// options is a set of optional options that can be set when a Migrate instance
// is created.
type options struct {
	// programmaticMigrations is a map of ProgrammaticMigration functions
	// that can be used to execute a Golang based migration step after a SQL
	// based migration step has been executed. The key is the migration
	// version and the value is the callback function that should be run
	// _after_ the step was executed (but within the same database
	// transaction).
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
// ProgrammaticMigration functions that can be used to execute a Golang based
// migration step after a SQL based migration step has been executed. The key is
// the migration version and the value is the programmatic migration function
// that should be run _after_ the step was executed (but before the version is
// marked as cleanly executed). An error returned from the programmatic
// migration will cause the migration to fail and the step to be marked as
// dirty.
func WithProgrammaticMigrations(pMigrs map[uint]ProgrammaticMigration) Option {
	return func(o *options) {
		o.programmaticMigrations = pMigrs
	}
}

// WithProgrammaticMigration is an option that can be used to set a
// ProgrammaticMigration function that can be used to execute a Golang based
// migration step after the SQL based migration step with the given version
// number has been executed. The programmatic migration is the function that
// should be run _after_ the step was executed (but before the version is marked
// as cleanly executed). An error returned from the programmatic migration will
// cause the migration to fail and the step to be marked as dirty.
func WithProgrammaticMigration(version uint,
	pMigr ProgrammaticMigration) Option {

	return func(o *options) {
		o.programmaticMigrations[version] = pMigr
	}
}
