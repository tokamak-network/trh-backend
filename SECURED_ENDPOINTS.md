# Secured Endpoints Documentation

This document describes all the secured endpoints in the TRH Backend API and their authentication requirements.

## Authentication Levels

### 1. **Public Endpoints** (No Authentication Required)
- Health check endpoints
- Login endpoint

### 2. **Authenticated Endpoints** (Valid JWT Token Required)
- Read-only operations
- User profile access

### 3. **Admin-Only Endpoints** (Admin Role Required)
- Stack management operations
- Integration management
- User management

## API Endpoints by Security Level

### 🔓 Public Endpoints

#### Health Check
```http
GET /api/v1/health
```

#### Authentication
```http
POST /api/v1/auth/login
Content-Type: application/json

{
    "email": "admin@tokamak.network",
    "password": "admin123"
}
```

### 🔐 Authenticated Endpoints (Require JWT Token)

#### User Profile
```http
GET /api/v1/auth/profile
Authorization: Bearer <jwt_token>
```

#### Stack Information (Read-Only)
```http
GET /api/v1/stacks/thanos
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/status
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/deployments
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/integrations
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/integrations/{integrationId}
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/deployments/{deploymentId}
Authorization: Bearer <jwt_token>

GET /api/v1/stacks/thanos/{id}/deployments/{deploymentId}/status
Authorization: Bearer <jwt_token>
```

### 🔒 Admin-Only Endpoints (Require Admin Role)

#### User Management
```http
GET /api/v1/auth/users
Authorization: Bearer <jwt_token>
```

#### Stack Management
```http
POST /api/v1/stacks/thanos
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "network": "Testnet",
    "l1RpcUrl": "https://...",
    "l1BeaconUrl": "https://...",
    "l2BlockTime": 2,
    "batchSubmissionFrequency": 10,
    "outputRootFrequency": 10,
    "challengePeriod": 7,
    "adminAccount": "0x...",
    "sequencerAccount": "0x...",
    "batcherAccount": "0x...",
    "proposerAccount": "0x...",
    "awsAccessKey": "...",
    "awsSecretAccessKey": "...",
    "awsRegion": "us-east-1",
    "chainName": "MyChain"
}

PUT /api/v1/stacks/thanos/{id}
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "l1RpcUrl": "https://...",
    "l1BeaconUrl": "https://..."
}

DELETE /api/v1/stacks/thanos/{id}
Authorization: Bearer <jwt_token>
```

#### Stack Control
```http
POST /api/v1/stacks/thanos/{id}/resume
Authorization: Bearer <jwt_token>

POST /api/v1/stacks/thanos/{id}/stop
Authorization: Bearer <jwt_token>
```

#### Integration Management
```http
POST /api/v1/stacks/thanos/{id}/integrations/bridge
Authorization: Bearer <jwt_token>

DELETE /api/v1/stacks/thanos/{id}/integrations/bridge
Authorization: Bearer <jwt_token>

POST /api/v1/stacks/thanos/{id}/integrations/block-explorer
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "databaseUsername": "postgres",
    "databasePassword": "password",
    "coinmarketcapKey": "your-key",
    "walletConnectId": "your-id"
}

DELETE /api/v1/stacks/thanos/{id}/integrations/block-explorer
Authorization: Bearer <jwt_token>

POST /api/v1/stacks/thanos/{id}/integrations/monitoring
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "grafanaPassword": "secure-password"
}

DELETE /api/v1/stacks/thanos/{id}/integrations/monitoring
Authorization: Bearer <jwt_token>

POST /api/v1/stacks/thanos/{id}/integrations/candidate-registry
Authorization: Bearer <jwt_token>
Content-Type: application/json

{
    "amount": 1000.0,
    "memo": "Registration memo",
    "nameInfo": "Candidate name"
}
```

## Authentication Flow

### 1. Login to Get JWT Token
```bash
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@tokamak.network",
    "password": "admin123"
  }'
```

**Response:**
```json
{
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
        "id": "uuid",
        "email": "admin@tokamak.network",
        "role": "Admin"
    }
}
```

### 2. Use JWT Token for Authenticated Requests
```bash
curl -X GET http://localhost:8000/api/v1/stacks/thanos \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

## Error Responses

### 401 Unauthorized
```json
{
    "error": "authorization header required"
}
```
```json
{
    "error": "invalid token"
}
```

### 403 Forbidden
```json
{
    "error": "admin access required"
}
```

## Security Features

1. **JWT Token Validation**: All protected endpoints validate JWT tokens
2. **Role-Based Access**: Admin endpoints check for admin role
3. **Token Expiration**: Tokens expire after 24 hours
4. **Secure Headers**: Authorization header required for all protected endpoints
5. **Input Validation**: All requests are validated before processing

## Testing with Swagger UI

1. Access Swagger UI at: `http://localhost:8000/swagger/index.html`
2. Click "Authorize" button
3. Enter your JWT token in the format: `Bearer <your-token>`
4. Test protected endpoints directly from the UI

## Environment Variables

Make sure to set these environment variables:

```env
JWT_SECRET=your-secret-key-change-in-production
DEFAULT_ADMIN_EMAIL=admin@yourdomain.com
DEFAULT_ADMIN_PASSWORD=your-secure-password
```

## Best Practices

1. **Always use HTTPS** in production
2. **Store JWT tokens securely** on the client side
3. **Implement token refresh** for long-running applications
4. **Log authentication events** for security monitoring
5. **Regularly rotate JWT secrets** in production
6. **Use strong passwords** for admin accounts 