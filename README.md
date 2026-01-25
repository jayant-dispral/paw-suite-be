# Brand Threat Backend

A comprehensive backend system for brand threat monitoring and social media threat detection using Go, Kubernetes, and Tilt for local development.

## Table of Contents

- [Project Overview](#project-overview)
- [Architecture](#architecture)
- [Tech Stack](#tech-stack)
- [Prerequisites](#prerequisites)
- [Local Development Setup](#local-development-setup)
- [Running the Application](#running-the-application)
- [API Endpoints](#api-endpoints)
- [Project Structure](#project-structure)
- [Services](#services)
- [Configuration](#configuration)
- [Development Workflow](#development-workflow)
- [Troubleshooting](#troubleshooting)

## Project Overview

Brand Threat is a backend system designed to:

- Monitor brand mentions across social media platforms
- Detect and alert on potential threats to brand reputation
- Manage projects with multiple monitoring keywords
- Provide user authentication and role-based access control
- Handle subscription tiers with different feature limits
- Store and analyze threat data in MongoDB

The system follows **Clean Architecture** principles with clear separation between domain, service, infrastructure, and handler layers.

## Architecture

### Clean Architecture Layers

```
┌─────────────────────────────────────┐
│     HTTP Handlers (Adapters)        │  ← User Interface
├─────────────────────────────────────┤
│     Service Layer (Use Cases)       │  ← Business Logic
├─────────────────────────────────────┤
│     Domain Layer (Interfaces)       │  ← Contracts
├─────────────────────────────────────┤
│  Infrastructure (Implementations)   │  ← Database, Events, gRPC
└─────────────────────────────────────┘
```

### Components

- **Admin Service**: Core microservice handling authentication, user management, and project management
- **MongoDB**: Primary data store for users, projects, and threat data
- **Nginx**: Reverse proxy and gateway for routing requests
- **Tilt**: Local Kubernetes development environment

## Tech Stack

| Component | Technology | Version |
|-----------|-----------|---------|
| Language | Go | 1.24.3 |
| Web Framework | Chi Router | v5.2.3 |
| Authentication | JWT | v5.3.0 |
| Database | MongoDB | 7.0 |
| Validation | Validator | v10.30.1 |
| Testing | Testify | v1.11.1 |
| Orchestration | Kubernetes | (via Tilt) |
| Local Dev | Tilt | - |

## Prerequisites

Before you begin, ensure you have the following installed:

### Required

- **Go**: 1.24.3 or higher
  ```bash
  go version
  ```

- **Docker**: 20.10+ and Docker Compose 1.29+
  ```bash
  docker --version
  docker compose --version
  ```

- **Kubernetes**: Kind cluster (for Tilt development)
  ```bash
  kind --version
  ```
  
  Create a kind cluster:
  ```bash
  kind create cluster --name paw-suite
  ```

- **Tilt**: For local development
  ```bash
  tilt --version
  ```
  
  [Install Tilt](https://docs.tilt.dev/install.html)

### Optional

- **MongoDB CLI tools** (for direct database access)
- **kubectl**: For Kubernetes management
  ```bash
  kubectl version --client
  ```

## Local Development Setup

### 1. Clone the Repository

```bash
cd /home/jayantissar/Desktop/Dispral
git clone <repository-url>
cd brand-threat-be
```

### 2. Install Go Dependencies

```bash
go mod download
go mod tidy
```

### 3. Setup Kind Kubernetes Cluster

```bash
# Create the kind cluster for Tilt
kind create cluster --name paw-suite

# Verify the cluster is running
kubectl cluster-info --context kind-paw-suite
```

### 4. Create Kubernetes Namespace (if needed)

```bash
kubectl create namespace paw-suite
```

### 5. Configure Environment Variables

The application reads configuration from environment variables. Default values are provided, but you can customize them:

**Default Configuration (in docker-compose.yml and Kubernetes manifests):**

```env
SERVER_PORT=8080
MONGODB_DATABASE_URI=mongodb://admin:password@mongo:27017/brand_threat?authSource=admin
MONGODB_DATABASE_NAME=brand_threat
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
APP_ENV=dev
```

For production, update these values in the K8s manifests.

## Running the Application

### Option 1: Using Tilt (Recommended for Local Development)

Tilt provides hot-reload development, automatic image building, and live Kubernetes updates.

```bash
# Start Tilt
tilt up

# This will:
# 1. Build the admin-service Docker image
# 2. Apply all Kubernetes manifests
# 3. Deploy MongoDB, admin-service, and Nginx
# 4. Open Tilt dashboard in your browser (localhost:10350)
```

**Tilt Dashboard Features:**
- View deployment status
- Stream logs from containers
- Trigger rebuilds
- See resource health

To stop Tilt:
```bash
tilt down
```

### Option 2: Using Docker Compose (Simpler Alternative)

For a quick local setup without Kubernetes:

```bash
# Start the services
docker compose up -d

# View logs
docker compose logs -f

# Access the API
curl http://localhost:3000/health
```

To stop:
```bash
docker compose down
```

### Option 3: Local Go Development

For debugging and rapid development:

```bash
# Terminal 1: Start MongoDB using Docker
docker run -d \
  --name mongo-dev \
  -p 27017:27017 \
  -e MONGO_INITDB_ROOT_USERNAME=admin \
  -e MONGO_INITDB_ROOT_PASSWORD=password \
  mongo:7.0

# Terminal 2: Run the admin-service
export SERVER_PORT=8080
export MONGODB_DATABASE_URI="mongodb://admin:password@localhost:27017/brand_threat?authSource=admin"
export MONGODB_DATABASE_NAME="brand_threat"
export JWT_SECRET="dev-secret-key"

cd services/admin-service
go run cmd/main.go
```

Server will start at `http://localhost:8080`

## API Endpoints

### Base URL
- **Local Dev**: `http://localhost:8080`
- **Kubernetes (via Nginx)**: `http://localhost:8080` (forwarded through port-forward)

### Authentication Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/auth/register` | Register a new user |
| `POST` | `/api/v1/auth/login` | Login and get JWT token |

**Register Example:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword",
    "full_name": "John Doe"
  }'
```

**Login Example:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "securepassword"
  }'
```

### User Endpoints (Requires Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/users/me` | Get current user profile |
| `POST` | `/api/v1/users/me` | Update user profile |

**Get Profile Example:**
```bash
curl -X GET http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <your-jwt-token>"
```

### Project Endpoints (Requires Authentication)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/projects` | Create a new project |
| `GET` | `/api/v1/projects` | List all user's projects |
| `GET` | `/api/v1/projects/{projectID}` | Get project details |
| `PUT` | `/api/v1/projects/{projectID}` | Update project details |
| `DELETE` | `/api/v1/projects/{projectID}` | Delete a project |
| `PUT` | `/api/v1/projects/{projectID}/status` | Update project status |
| `PUT` | `/api/v1/projects/{projectID}/monitoring` | Update monitoring config |
| `PUT` | `/api/v1/projects/{projectID}/alerts` | Update alert config |
| `POST` | `/api/v1/projects/{projectID}/team` | Add team member |
| `PUT` | `/api/v1/projects/{projectID}/team/{userId}` | Update team member role |

### Health Check

```bash
curl http://localhost:8080/health
```

## Project Structure

```
brand-threat-be/
├── docker-compose.yml              # Docker Compose for local development
├── Dockerfile                       # Multi-stage Docker build for admin-service
├── Makefile                         # Build and test automation
├── Tiltfile                         # Tilt configuration for K8s development
├── go.mod                           # Go module dependencies
├── infra/
│   └── k8s/                         # Kubernetes manifests
│       ├── admin-service/           # Admin service deployment
│       ├── mongo/                   # MongoDB deployment
│       └── nginx/                   # Nginx gateway deployment
├── services/
│   ├── admin-service/               # Main microservice
│   │   ├── cmd/
│   │   │   └── main.go              # Service entry point
│   │   ├── internal/
│   │   │   ├── adapters/            # External adapters (HTTP handlers, repos)
│   │   │   │   ├── handler/         # HTTP request handlers
│   │   │   │   └── repository/      # Database repositories (MongoDB)
│   │   │   ├── config/              # Configuration management
│   │   │   ├── domain/              # Business domain models and interfaces
│   │   │   ├── service/             # Business logic implementation
│   │   │   │   ├── auth/            # Authentication logic
│   │   │   │   ├── project/         # Project management logic
│   │   │   │   └── user/            # User management logic
│   │   │   └── infrastructure/      # External services (events, gRPC)
│   │   └── pkg/
│   │       └── types/               # Shared public types
│   └── data-service/                # Data processing microservice (planned)
└── shared/
    ├── domain/                      # Shared domain models
    │   ├── alert.go
    │   ├── project.go
    │   ├── threat.go
    │   ├── user.go
    │   ├── audit_log.go
    │   ├── errors.go
    │   ├── metrics.go
    │   └── social_post.go
    └── pkg/
        ├── errors/                  # Shared error handling
        ├── http/                    # Shared HTTP utilities
        ├── jwt/                     # JWT utilities
        ├── response/                # HTTP response formatting
        └── validator/               # Shared validators
```

## Services

### Admin Service

**Purpose**: Core service for user management, authentication, and project administration.

**Components:**
- **Authentication Service**: JWT-based user authentication and token generation
- **User Service**: User profile management
- **Project Service**: CRUD operations for projects and monitoring configurations

**Technology Stack:**
- Go 1.24.3
- Chi HTTP Router
- JWT authentication
- MongoDB for persistence

**Port**: 8080 (internally), 3000 (docker-compose)

### Data Service (Planned)

Infrastructure and placeholders exist for a data service to handle threat detection and analysis.

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | 8080 | HTTP server port |
| `MONGODB_DATABASE_URI` | mongodb://admin:password@mongo:27017/brand_threat?authSource=admin | MongoDB connection string |
| `MONGODB_DATABASE_NAME` | brand_threat | Database name |
| `JWT_SECRET` | your-super-secret-jwt-key-change-this-in-production | JWT signing secret |
| `APP_ENV` | dev | Application environment (dev/staging/prod) |

### MongoDB

**Default Credentials:**
- Username: `admin`
- Password: `password`
- Database: `brand_threat`

**Collections:**
- `users` - User accounts and subscriptions
- `projects` - Project configurations
- `threats` - Detected threats
- `audit_logs` - Audit trail

### Subscription Tiers

| Tier | Max Projects | Max Keywords | Data Retention | API Calls/Month |
|------|--------------|--------------|-----------------|-----------------|
| Free | 1 | 5 | 7 days | 1,000 |
| Pro | 10 | 50 | 90 days | 50,000 |
| Enterprise | 100 | 500 | 365 days | 1,000,000 |

## Development Workflow

### 1. Making Code Changes

When using Tilt:
1. Edit code in your IDE
2. Save files
3. Tilt automatically detects changes
4. Docker image rebuilds
5. New image deployed to Kubernetes
6. View logs in Tilt dashboard

### 2. Running Tests

```bash
# Run all tests with coverage
make test

# Generate HTML coverage report
make test-html

# Run specific test package
go test ./services/admin-service/internal/service/...
```

### 3. Code Formatting and Linting

```bash
# Format code
make fmt

# Run vet check
make vet
```

### 4. Building for Production

```bash
# Build Docker image
docker build -t admin-service:latest \
  -f services/admin-service/Dockerfile \
  .

# Tag for registry
docker tag admin-service:latest your-registry/admin-service:latest

# Push to registry
docker push your-registry/admin-service:latest
```

### 5. Updating Kubernetes Manifests

Edit YAML files in `infra/k8s/` and apply:

```bash
kubectl apply -f infra/k8s/admin-service/deployment.yaml
kubectl apply -f infra/k8s/mongo/deployment.yaml
kubectl apply -f infra/k8s/nginx/
```

## Kubernetes Management

### View Deployments

```bash
# List all resources in the namespace
kubectl get all -n paw-suite

# Describe a specific deployment
kubectl describe deployment admin-service -n paw-suite

# View pod logs
kubectl logs -f deployment/admin-service -n paw-suite

# Port forward to access locally
kubectl port-forward svc/admin-service 8080:8080 -n paw-suite
```

### Scale Services

```bash
# Scale admin-service to 3 replicas
kubectl scale deployment admin-service --replicas=3 -n paw-suite
```

## Troubleshooting

### Issue: Tilt won't start or cluster not found

```bash
# Check if kind cluster exists
kind get clusters

# If not, create it
kind create cluster --name paw-suite

# Set kubectl context
kubectl config use-context kind-paw-suite
```

### Issue: MongoDB connection refused

```bash
# Verify MongoDB is running
kubectl get pods -n paw-suite | grep mongo

# Check MongoDB logs
kubectl logs -f deployment/mongo -n paw-suite

# Port forward to test locally
kubectl port-forward svc/mongo 27017:27017 -n paw-suite

# Test connection
mongosh "mongodb://admin:password@localhost:27017/brand_threat?authSource=admin"
```

### Issue: Admin-service fails to start

```bash
# Check logs
kubectl logs -f deployment/admin-service -n paw-suite

# Verify config is set
kubectl get deployment admin-service -n paw-suite -o yaml | grep -A 20 "env:"

# Check if image is built
docker images | grep admin-service
```

### Issue: Port conflicts

If ports are already in use:

```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port in config
```

### Clean Up

```bash
# Stop Tilt
tilt down

# Delete kind cluster
kind delete cluster --name paw-suite

# Clean Docker resources
docker compose down -v
```

## Production Deployment

### Before deploying to production:

1. **Update JWT Secret**:
   ```bash
   kubectl set env deployment/admin-service JWT_SECRET="<strong-random-secret>" -n paw-suite
   ```

2. **Update MongoDB credentials** in K8s secrets

3. **Configure proper CORS origins** in router.go:
   ```go
   AllowedOrigins: []string{"https://yourdomain.com"},
   ```

4. **Enable HTTPS** with certificate management

5. **Set up monitoring and logging** (Prometheus, ELK, etc.)

6. **Create database backups** regularly

7. **Implement rate limiting** for API endpoints

## Contributing

1. Create a feature branch
2. Make your changes
3. Run tests: `make test`
4. Format code: `make fmt`
5. Run linter: `make vet`
6. Commit and push
7. Create a pull request

## Support

For issues or questions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review logs in Tilt dashboard or via `kubectl logs`
3. Check Kubernetes event logs: `kubectl describe pod <pod-name> -n paw-suite`

## License

[Your License Here]

---

**Last Updated**: January 2026
**Go Version**: 1.24.3
**Kubernetes**: 1.28+
