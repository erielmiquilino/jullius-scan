## ADDED Requirements

### Requirement: Containerized service topology
The system SHALL run the backend platform on a VPS using Docker Compose with separately addressable services for the API, scraping worker, and PostgreSQL database, while Redis is provided as an internally reachable VPS container managed manually for MVP queueing.

#### Scenario: Start platform services on VPS
- **WHEN** the deployment stack is started on the target VPS
- **THEN** the API, worker, and PostgreSQL services are launched as containers within the composed application topology

#### Scenario: Reach manually provisioned Redis
- **WHEN** the API and worker containers start on the VPS
- **THEN** they can connect to the manually provisioned Redis container through the shared internal network configuration

#### Scenario: Isolate worker from public ingress
- **WHEN** the scraping worker service is deployed
- **THEN** it is available only on the internal container network and is not exposed directly to public traffic

### Requirement: Traefik-based ingress routing
The system SHALL use Traefik as the reverse proxy, with Docker labels defining how public HTTP(S) traffic is routed to the API service.

#### Scenario: Route public API traffic through Traefik
- **WHEN** an external client calls the configured API host on the VPS
- **THEN** Traefik routes the request to the API container according to the declared labels

#### Scenario: Keep database off public ingress
- **WHEN** the deployment is inspected for exposed routes
- **THEN** PostgreSQL is not reachable through Traefik public routing

### Requirement: CI/CD delivery through GitHub Actions and local registry
The system SHALL support a deployment pipeline in which GitHub Actions builds service images, publishes them to the existing registry at `https://registry.skadi.digital/`, and updates the running stack over SSH using versioned images from that registry.

#### Scenario: Publish versioned images from CI
- **WHEN** the CI pipeline runs for a deployable revision
- **THEN** it builds the required images and publishes versioned artifacts to `https://registry.skadi.digital/` over SSL

#### Scenario: Deploy updated stack from registry
- **WHEN** the VPS deployment process pulls a new approved image version from `https://registry.skadi.digital/`
- **THEN** the corresponding service is updated without requiring manual image transfer to the host

#### Scenario: Authenticate deploy over SSH key
- **WHEN** GitHub Actions performs a deployment to the VPS
- **THEN** it connects using the existing SSH `.pem` key material instead of interactive credentials

### Requirement: Environment-based operational configuration
The system SHALL configure runtime secrets and environment-specific values separately from the container images.

#### Scenario: Inject environment configuration at deploy time
- **WHEN** the platform is deployed in a target environment
- **THEN** Firebase, database, browser, and routing configuration values are supplied through environment or secret configuration rather than hardcoded in images
