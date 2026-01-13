# Project Routes Design

This document outlines the API routes for project management in the Sentinel Brand Threat Management Platform.

## Base Path
All routes are under `/api/v1/projects` and require authentication via JWT token.

## Route List

----------Done-------------------
### 1. Create Project
- **Method**: POST
- **Path**: `/`
- **Description**: Creates a new project for the authenticated user.
- **Request Body**:
  ```json
  {
    "name": "My Brand Project",
    "description": "Monitoring my brand",
    "brand_name": "MyBrand",
    "primary_domain": "mybrand.com",
    "official_handles": [
      {
        "platform": "twitter",
        "handle": "@mybrand",
        "verified": true
      }
    ],
    "monitoring_config": {
      "keywords": ["mybrand"],
      "hashtags": ["#mybrand"],
      "handles": ["@mybrand"],
      "platforms": ["twitter"],
      "scan_frequency": 15,
      "enable_typosquatting": true,
      "enable_impersonating": true,
      "typosquatting_scan_freq": 24
    },
    "alert_config": {
      "negative_spike_threshold": 10,
      "spike_window_minutes": 5,
      "viral_threshold": 100,
      "email_recipients": ["alerts@mybrand.com"],
      "slack_web_hook_url": "https://hooks.slack.com/...",
      "quiet_hours_start": "22:00",
      "quite_hours_end": "08:00"
    }
  }
  ```
- **Response**: 201 Created with project data

----------Done-------------------
### 2. List Projects
- **Method**: GET
- **Path**: `/`
- **Description**: Retrieves all projects owned by or accessible to the authenticated user.
- **Query Params**: `status` (optional: active, paused, archived), `limit`, `offset`
- **Response**: 200 OK with array of projects




### 3. Get Project by ID
- **Method**: GET
- **Path**: `/{id}`
- **Description**: Retrieves a specific project by its ID. User must be owner or team member.
- **Response**: 200 OK with project data

### 4. Update Project
- **Method**: PUT
- **Path**: `/{id}`
- **Description**: Updates project details (name, description, brand info). User must be owner or admin team member.
- **Request Body**: Partial project data (same as create but optional fields)
- **Response**: 200 OK with updated project

### 5. Delete Project
- **Method**: DELETE
- **Path**: `/{id}`
- **Description**: Archives the project (soft delete). User must be owner.
- **Response**: 200 OK

### 6. Update Project Status
- **Method**: PUT
- **Path**: `/{id}/status`
- **Description**: Changes project status (active/paused/archived). User must be owner or admin.
- **Request Body**:
  ```json
  {
    "status": "paused"
  }
  ```
- **Response**: 200 OK

### 7. Update Monitoring Config
- **Method**: PUT
- **Path**: `/{id}/monitoring`
- **Description**: Updates monitoring configuration. User must be owner or editor.
- **Request Body**: MonitoringConfig object
- **Response**: 200 OK

### 8. Update Alert Config
- **Method**: PUT
- **Path**: `/{id}/alerts`
- **Description**: Updates alert configuration. User must be owner or editor.
- **Request Body**: AlertConfig object
- **Response**: 200 OK

### 9. Add Team Member
- **Method**: POST
- **Path**: `/{id}/team`
- **Description**: Adds a team member to the project. User must be owner or admin.
- **Request Body**:
  ```json
  {
    "user_id": "user_object_id",
    "role": "editor"
  }
  ```
- **Response**: 201 Created

### 10. Update Team Member Role
- **Method**: PUT
- **Path**: `/{id}/team/{userId}`
- **Description**: Updates a team member's role. User must be owner or admin.
- **Request Body**:
  ```json
  {
    "role": "admin"
  }
  ```
- **Response**: 200 OK

### 11. Remove Team Member
- **Method**: DELETE
- **Path**: `/{id}/team/{userId}`
- **Description**: Removes a team member from the project. User must be owner or admin.
- **Response**: 200 OK

### 12. List Team Members
- **Method**: GET
- **Path**: `/{id}/team`
- **Description**: Lists all team members for the project.
- **Response**: 200 OK with array of team members

## Authorization Notes
- Owner: Full access
- Admin: Can manage team, update configs, change status
- Editor: Can update configs
- Viewer: Read-only access

## Error Responses
- 400 Bad Request: Validation errors
- 401 Unauthorized: Invalid/missing token
- 403 Forbidden: Insufficient permissions
- 404 Not Found: Project not found
- 409 Conflict: Duplicate data (e.g., adding existing team member)