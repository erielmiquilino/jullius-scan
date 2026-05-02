### Requirement: Atomic user provisioning into Firebase and database
The system SHALL provide an administrative command-line tool that, given an email and password, atomically provisions a new House member by creating a Firebase Auth user (when one does not yet exist for the email), inserting the corresponding row into the application database, and linking the user to a target House membership.

#### Scenario: Provision a brand-new user
- **WHEN** the operator runs the CLI with a valid email, password, House identifier, and a Firebase service account that has user-management permissions, and no Firebase user exists for that email and no `users` row exists for the resulting Firebase UID
- **THEN** the CLI creates the user in Firebase Auth with the given email and password, inserts a `users` row carrying that Firebase UID and email, inserts a `house_members` row linking the user to the target House, and reports success including the resolved Firebase UID and `users.id`

#### Scenario: Reject provisioning without a target House
- **WHEN** the operator runs the CLI without specifying a House (neither a House id nor a House name resolvable to exactly one row in `houses`)
- **THEN** the CLI exits with a non-zero status, prints an explanatory message, does not call Firebase Auth, and does not write to the database

#### Scenario: Reject provisioning when service account lacks permissions
- **WHEN** the configured Firebase service account does not have permission to create users
- **THEN** the CLI surfaces the authorization error from the Firebase Admin SDK, exits with non-zero status, and does not write to the database

### Requirement: Idempotent re-runs converge to a consistent state
The system SHALL allow the provisioning CLI to be invoked repeatedly for the same email and House without creating duplicate Firebase users, duplicate `users` rows, or duplicate `house_members` rows, and without failing when any of those records already exist.

#### Scenario: Re-run for an existing Firebase user
- **WHEN** the operator runs the CLI for an email that already has a Firebase Auth user but no corresponding `users` row in the database
- **THEN** the CLI reuses the existing Firebase UID without attempting to recreate the user, inserts the missing `users` row using that UID, ensures the `house_members` membership is present, and reports success indicating that the Firebase user was preexisting

#### Scenario: Re-run for a fully provisioned user
- **WHEN** the operator runs the CLI for an email whose Firebase user, `users` row, and `house_members` row already exist for the target House
- **THEN** the CLI completes successfully without modifying any data and reports that the user is already provisioned for that House

#### Scenario: Password is ignored when Firebase user already exists
- **WHEN** the operator runs the CLI with a password for an email that already corresponds to an existing Firebase user
- **THEN** the CLI does not change the existing Firebase password, logs a warning that the supplied password was ignored, and continues with the database provisioning steps

### Requirement: House membership scoping
The system SHALL ensure that any membership created by the provisioning CLI is associated with exactly one explicitly chosen House, and SHALL reject ambiguous or unresolved House selections.

#### Scenario: Resolve House by id
- **WHEN** the operator runs the CLI with a House id that exists in `houses`
- **THEN** the CLI links the new (or reused) user to that exact House id

#### Scenario: Resolve House by unique name
- **WHEN** the operator runs the CLI with a House name that matches exactly one `houses.name` row
- **THEN** the CLI links the new (or reused) user to the matched House

#### Scenario: Reject ambiguous House name
- **WHEN** the operator runs the CLI with a House name that matches more than one `houses` row
- **THEN** the CLI exits with a non-zero status, surfaces the ambiguity, and does not create a Firebase user nor any database rows

### Requirement: Secure password handling for the CLI
The system SHALL accept the new user's password through a mechanism that does not require the operator to expose the password in process listings or shell history.

#### Scenario: Accept password from standard input
- **WHEN** the operator invokes the CLI with the password-from-stdin option and pipes the password to the process
- **THEN** the CLI reads the password from standard input, never logs the password value, and proceeds with the provisioning flow

#### Scenario: Never log password value
- **WHEN** the CLI emits structured logs during a successful or failed run
- **THEN** the password value SHALL NOT appear in any log line, error message, or telemetry output produced by the CLI
