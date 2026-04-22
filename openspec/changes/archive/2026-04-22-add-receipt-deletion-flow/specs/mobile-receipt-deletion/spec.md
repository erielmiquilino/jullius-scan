## ADDED Requirements

### Requirement: Receipt detail offers destructive removal flow
The mobile app SHALL allow the user to remove a persisted Scan/receipt from the receipt detail screen through an explicit destructive action.

#### Scenario: Show delete action on detail screen
- **WHEN** the user opens the detail screen of an existing receipt
- **THEN** the screen shows an action that allows starting the removal flow for that receipt

### Requirement: Receipt deletion requires explicit confirmation
The mobile app SHALL ask the user to confirm the removal before calling the deletion API.

#### Scenario: Confirm removal before API call
- **WHEN** the user taps the delete action on the receipt detail screen
- **THEN** the app shows a confirmation dialog asking if the user really wants to remove the record before executing the deletion

#### Scenario: Cancel removal from confirmation dialog
- **WHEN** the user dismisses or cancels the confirmation dialog
- **THEN** the app keeps the receipt detail screen open and does not call the deletion API

### Requirement: Successful deletion returns to refreshed list
The mobile app SHALL return to the main receipt list after a successful deletion and refresh the list so the removed record is no longer shown.

#### Scenario: Delete and go back to updated list
- **WHEN** the user confirms the removal and the API deletion succeeds
- **THEN** the app closes the detail screen, returns to the list screen, and reloads the receipts without the removed item

### Requirement: Failed deletion keeps user on detail screen
The mobile app SHALL keep the user on the detail screen and show an error if the deletion request fails.

#### Scenario: API deletion error
- **WHEN** the user confirms the removal and the API responds with an error
- **THEN** the app shows the failure feedback, stops the loading state, and keeps the current receipt detail screen visible
