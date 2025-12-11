package migrate

import "github.com/golang-migrate/migrate/v4/database"

// ProgrammaticMigration is a Golang function type that can be used to execute
// a Golang-based migration. The Golang function receives the migration and
// the database driver as arguments.
type ProgrammaticMigration func(migr *Migration, driver database.Driver) error

// ProgrammaticMigrEntry configures how a programmatic migration should run.
type ProgrammaticMigrEntry struct {
	// ResetVersionOnError determines the behavior if the programmatic
	// migration errors. If set to `true` the migration logic will reset the
	// database to the prior version, so the migration is retried on the
	// next startup.
	// If set to `false`, the database will be left in dirty state at the
	// migration version of the programmatic migration.
	//
	// NOTE: If this is set to true, the ProgrammaticMigration's
	// implementation should account for the fact that it may be re-executed
	// on the next startup if it errors. This could, for example, be done by
	// executing the entire migration within a single database transaction,
	// which is not committed if an error occurs.
	ResetVersionOnError bool

	// ProgrammaticMigr is the holds the Golang-based migration function.
	ProgrammaticMigr ProgrammaticMigration
}

// options is a set of optional parameters that can be set when creating a
// Migrate instance.
type options struct {
	// programmaticMigrations is a map of ProgrammaticMigrEntry objects
	// that can be used to execute a Golang-based migration. The key is the
	// migration version, and the value is the ProgrammaticMigrEntry.
	programmaticMigrations map[uint]ProgrammaticMigrEntry
}

// defaultOptions returns a new options struct with default values.
func defaultOptions() options {
	return options{
		programmaticMigrations: make(map[uint]ProgrammaticMigrEntry),
	}
}

// Option is a function that can be used to set options on a Migrate instance.
type Option func(*options)

// WithProgrammaticMigrations is an option that can be used to set a map of
// ProgrammaticMigrEntry objects that can be used to execute a Golang-based
// migration step. The key is the migration version, and the value is the
// ProgrammaticMigrEntry.
// The ProgrammaticMigrEntry also defines the behavior if the Golang-based
// migration errors:
// * One option is that the migration fails but does **not** set the database
// to a dirty state. Additionally, if the programmatic migration fails, the
// database version will be reset to the last set version before executing the
// programmatic migration. The effect of this is that the programmatic migration
// will be re-run on the next startup of the database.
// * The other alternative is that the database will be left in dirty state at
// the migration version of the programmatic migration.
func WithProgrammaticMigrations(pMigrs map[uint]ProgrammaticMigrEntry) Option {
	return func(o *options) {
		for version, pMigr := range pMigrs {
			WithProgrammaticMigration(version, pMigr)(o)
		}
	}
}

// WithProgrammaticMigration is an option that can be used to set a
// ProgrammaticMigration object that can be used to execute a Golang-based
// migration.
// The ProgrammaticMigrEntry also defines the behavior if the Golang-based
// migration errors:
// * One option is that the migration fails but does **not** set the database
// to a dirty state. Additionally, if the programmatic migration fails, the
// database version will be reset to the last set version before executing the
// programmatic migration. The effect of this is that the programmatic migration
// will be re-run on the next startup of the database.
// * The other alternative is that the database will be left in dirty state at
// the migration version of the programmatic migration.
func WithProgrammaticMigration(version uint,
	pMigr ProgrammaticMigrEntry) Option {

	return func(o *options) {
		o.programmaticMigrations[version] = pMigr
	}
}
