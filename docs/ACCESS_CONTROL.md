# Access Control System

This document describes the Role-Based Access Control (RBAC) system implemented using go-casbin and PostgreSQL.

## Overview

The access control system provides fine-grained permissions for different user roles across all operations in the import-export backend system.

## User Roles

### 1. Admin

- **Permissions**: Full access to all operations
- **Operations**: view/create/update/delete for all resources + view prices

### 2. Accountant

- **Permissions**: Limited access focused on financial operations
- **Operations**:
  - view/create/update/delete purchase-orders
  - view only for all other resources
  - view prices

### 3. Staff

- **Permissions**: Read-only access
- **Operations**: view only for all resources except prices

### 4. Developer

Standalone maintenance role. It is **not** derived from `admin` and does not imply it,
so an account holding `developer` sees exactly one screen and can call only the
developer tool endpoints. It holds exactly three permissions:

| permission | purpose |
|---|---|
| `developer-tools:view` | Screen visibility only; not backed by any route. The frontend gates the sidebar entry and page guard on this string and nothing else. |
| `initial-stock-import:import` | All three initial-stock tool endpoints (`GET /tools/inventories`, `POST /tools/initial-stock/sheets`, `POST /tools/initial-stock/import`). |
| `permissions:view` | `GET /users/permissions`. Without it the account 403s on its first call and the UI renders a full-screen error panel, making the account unusable. |

`inventories:view` is deliberately **not** granted: it maps to both the Inventory and
Inventory Timeline screens in the frontend, which would give a standalone developer
three sidebar entries. The tool supplies its own inventory list instead.

Because `users.role` holds one role, an account assigned `developer` is developer-only
while the assignment lasts. The update endpoint below rejects `developer`, so it is
assigned by migration.

## Role Assignment and Revocation

Each user holds exactly one role, in the `users.role` column. Authorization passes it
to Casbin as the subject, and the `p, <role>, <resource>, <action>, <effect>` rows in
`rbac_policy.csv` define what that role can do.

- **Assign a role**: the Users screen, backed by `PUT /api/v1/users/:id` (admin only).
  `:id` is the `users.id` UUID, not the Firebase UID, and the body is the whole record —
  `{"name": …, "role": …, "status": …}`, with `role` and `status` both required.
  Effective on the user's next request. `role` accepts only `admin`, `accountant`,
  `staff` and `bot_form` (`dto.UpdateUserRequest`); every other role, including
  `developer` and `restaurant-admin`, is rejected with 400 and must be set by migration.

  The call **returns 500 even when it succeeds**: the row is written, then the path UUID
  is passed to Firebase as a UID and rejected (`user_handler.go:123-137`). Check the
  record instead of retrying.
- **Revoke access**: set `status` to `inactive` or `pending`, checked at
  `authorization.go:45` before any Casbin call, so it applies on the next request.
  Disabling the Firebase account does **not** revoke: `VerifyToken` calls
  `VerifyIDToken` (`firebase_auth.go:49`), which ignores revocation and disabled state,
  so an unexpired token keeps working until it expires. Neither needs a deploy.
- **Change what a role can do**: edit its `p` rows. `rbac_policy.csv` is baked into the
  image (`Dockerfile:42-43`) and loaded once at boot
  (`internal/auth/casbin_service.go:26-38`), so this requires a redeploy.

Do not add `g, <uid>, <role>` rows: they grant no API access and desynchronise the
sidebar from it.

## Resources and Actions

The system defines the following resources:

- `products` - Product management
- `suppliers` - Supplier management
- `inventories` - Inventory management
- `purchase_orders` - Purchase order management
- `excel` - Excel import/export operations
- `settings` - System settings
- `prices` - Pricing information and reports
- `developer-tools` - Developer tool screen visibility
- `initial-stock-import` - Opening-stock loader endpoints

Actions available:

- `view` - Read/GET operations
- `create` - POST operations
- `update` - PUT/PATCH operations
- `delete` - DELETE operations

## Implementation Details

### Casbin Configuration

The RBAC model is defined in `internal/auth/rbac_model.conf`:

```
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

### Policy Rules

Default policies are defined in `internal/auth/rbac_policy.csv` and automatically loaded into the database.

### Authorization Middleware

The `AuthorizationMiddleware` in `internal/middleware/authorization.go`:

1. Extracts user role from the authentication context
2. Maps HTTP methods to actions (GET→view, POST→create, etc.)
3. Maps URL paths to resources
4. Checks permissions using Casbin enforcer
5. Allows or denies access based on the result

### User Management

Users are managed through:

- `internal/models/user.go` - User model with role field
- `internal/repository/user_repository.go` - Database operations
- `internal/services/user_service.go` - Business logic
- `internal/handlers/user_handler.go` - HTTP endpoints

## API Endpoints

### Authentication

- `POST /api/v1/auth/verify-token` - Verify Firebase token and create/update user
- `GET /api/v1/auth/profile` - Get current user profile

### User Management (Admin only)

- `GET /api/v1/users` - List all users
- `GET /api/v1/users/search` - Search users by name or email
- `GET /api/v1/users/role/:role` - Get users by role
- `PUT /api/v1/users/:id` - Update a user (name, role, status)
- `DELETE /api/v1/users/:id` - Delete user

## Usage Examples

### 1. Setting up a new user

```bash
# Create user with default staff role
curl -X POST "http://localhost:8080/api/v1/auth/verify-token" \
  -H "Content-Type: application/json" \
  -d '{"token": "firebase-id-token"}'
```

### 2. Updating user role (Admin only)

```bash
# Promote user to admin. Path parameter is the users.id UUID. Returns 500 on success;
# see Role Assignment and Revocation.
curl -X PUT "http://localhost:8080/api/v1/users/550e8400-e29b-41d4-a716-446655440000" \
  -H "Authorization: Bearer admin-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "John Doe", "role": "admin", "status": "active"}'
```

### 3. Testing permissions

```bash
# Test accountant creating purchase order (should work)
curl -X POST "http://localhost:8080/api/v1/purchase-orders" \
  -H "Authorization: Bearer accountant-token" \
  -H "Content-Type: application/json" \
  -d '{"supplier_id": "123", "items": []}'
```

## Database Schema

### Users Table

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uid VARCHAR UNIQUE NOT NULL,
    email VARCHAR UNIQUE NOT NULL,
    name VARCHAR,
    role VARCHAR DEFAULT 'staff',
    active BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);
```

### Casbin Tables

Casbin automatically creates the following tables:

- `casbin_rule` - Stores RBAC policies

## Seeding Default Users

Run the seed command to create default users:

```bash
go run cmd/util/main.go seed-users
```

This creates:

- Admin user (admin@example.com)
- Accountant user (accountant@example.com)
- Staff user (staff@example.com)

## Testing

Use the provided test script to verify access control:

```bash
./test/test_access_control.sh
```

## Security Considerations

1. **Token Validation**: All requests require valid Firebase ID tokens
2. **Role Synchronization**: User roles are synchronized between database and Firebase custom claims
3. **Policy Persistence**: RBAC policies are stored in PostgreSQL for persistence
4. **Audit Trail**: All authorization decisions are logged
5. **Default Deny**: Access is denied by default unless explicitly allowed

## Troubleshooting

### Common Issues

1. **403 Forbidden**: Check user role and permissions
2. **401 Unauthorized**: Verify Firebase token is valid
3. **Policy not found**: Ensure policies are initialized on startup

### Debugging

Enable debug logging to see authorization decisions:

```go
logger.SetLevel(logrus.DebugLevel)
```

## Future Enhancements

1. **Resource-level permissions**: Fine-grained control per resource instance
2. **Time-based permissions**: Temporary role assignments
3. **Permission inheritance**: Hierarchical role structure
4. **Audit logging**: Detailed access logs for compliance
