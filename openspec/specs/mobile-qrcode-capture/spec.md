### Requirement: QR code scanning from camera
The mobile app SHALL provide an in-app camera-based QR code scanner that recognizes NFC-e fiscal URLs and submits them to the backend on behalf of the authenticated user.

#### Scenario: Authenticated user scans a valid NFC-e QR code
- **WHEN** an authenticated user opens the scanner screen and points the camera at a valid NFC-e QR code
- **THEN** the app decodes the URL, submits it to the backend receipt submission endpoint with the Firebase bearer token, and navigates to the job tracking view with the returned job identifier

#### Scenario: QR code does not match an NFC-e URL pattern
- **WHEN** the scanner decodes a QR code whose content does not match the expected NFC-e fiscal URL pattern
- **THEN** the app displays an inline validation error to the user and does not submit the content to the backend

### Requirement: Camera permission handling
The mobile app SHALL request and gracefully handle the camera permission required by the QR scanner.

#### Scenario: Camera permission is denied
- **WHEN** the user denies the camera permission request on the scanner screen
- **THEN** the app displays an explanation with an action to open system settings, and does not crash or leave the scanner in a frozen state

#### Scenario: Camera permission is granted after previous denial
- **WHEN** the user grants the camera permission after initially denying it
- **THEN** the scanner becomes functional without requiring the user to restart the app

### Requirement: Manual fallback entry
The mobile app SHALL allow the authenticated user to manually paste an NFC-e URL as a fallback when QR scanning is not available or fails.

#### Scenario: Manual URL submission from scanner screen
- **WHEN** the user chooses the manual entry option from the scanner screen and submits a valid NFC-e URL
- **THEN** the app performs the same submission flow as a scanned QR code and navigates to the job tracking view
