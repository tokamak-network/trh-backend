# Preset-Based Deployment Flow: Spec Compliance Review

**Document ID**: TRH-PRESET-SPEC-001  
**Last Updated**: 2026-04-16  
**Status**: Active (11 Issues Tracked)

---

## Overview

This document specifies the complete preset-based deployment flow from frontend UI through backend services. It covers:

1. Frontend form handling and validation (Zod schemas)
2. API communication and error handling
3. Backend deployment orchestration
4. Known issues and improvement areas

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    Frontend (trh-platform-ui)               │
├─────────────────────────────────────────────────────────────┤
│  Step 1: PresetSelectionStep (Choose Preset)               │
│    ↓                                                         │
│  Step 2: BasicInfoStep (Form Validation + Input)           │
│    • form.trigger("presetBasicInfo") [Zod validation]      │
│    • Frontend validation: chainName, seedPhrase, etc.       │
│    ↓                                                         │
│  Step 3: goToNextStep() + currentStep === 3 check          │
│    • Conditional dispatch: await handleDeploy()            │
│    ↓                                                         │
│  handleDeploy() → startPresetDeployment(request)            │
│    • Calls apiPost("stacks/thanos/preset-deploy", request) │
└─────────────────────────────────────────────────────────────┘
                           ↓
           Request Interceptor (api.ts line 26-41)
           • Authorization header injection
           • Token retrieval from localStorage
                           ↓
        ┌────────────────────────────────────────┐
        │  API Proxy: /api/proxy/...             │
        │  → Rewritten to trh-backend:8000       │
        └────────────────────────────────────────┘
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                    Backend (trh-backend)                    │
├─────────────────────────────────────────────────────────────┤
│  POST /stacks/thanos/preset-deploy                          │
│    ↓                                                         │
│  PresetDeploy Handler (presets.go line 78-104)             │
│    • ShouldBindJSON() - Request deserialization            │
│    • ValidateProvider() [Backend validation]               │
│    • CreateThanosStackFromPreset()                         │
│    ↓                                                         │
│  Response Generation                                        │
│    ⚠️ ERROR: No response on error (lines 99-101)           │
│    • 200 OK: PresetDeploymentResponse with deploymentId    │
│    • 400: Bad request (validation failure)                 │
│    • 500: Server error (not returned - BUG)                │
└─────────────────────────────────────────────────────────────┘
                           ↓
        Response Interceptor (api.ts line 44-64)
        • 200 OK: Pass through
        • 401: Remove accessToken from localStorage
        • 403: Log to console.error()
        • Others: Promise.reject()
                           ↓
┌─────────────────────────────────────────────────────────────┐
│         Frontend Error Handling (handleDeploy)              │
│         • catch(error) → toast.error(message)              │
│         • UI displays error notification                    │
└─────────────────────────────────────────────────────────────┘
```

---

## Step 1: Preset Selection (Line 45-51 in page.tsx)

**Component**: `PresetSelectionStep`

**Behavior**:
- Displays 4 preset cards: General, DeFi, Gaming, Full
- User selects one preset
- `handleSelectPreset(preset)` called (usePresetWizard.ts line 71-79)
  - Updates selected preset in context
  - Sets default feeToken via `getPresetUIMetadata()`
  - Triggers validation

**Exit Condition**: Click "Next" button (disabled if no preset selected)

---

## Step 2: Basic Info Form (Lines 52-57 in page.tsx, Lines 116-127 in page.tsx for button)

**Component**: `BasicInfoStep`

**Frontend Zod Validation** (create-rollup.ts lines 193-229):

```typescript
presetBasicInfoSchema = z.object({
  chainName: string               // regex: ^[a-z0-9-]{3,32}$
  network: enum(["Testnet", "Mainnet"])
  infraProvider: enum(["aws", "local"])
  l1RpcUrl: string                // Must be valid URL
  l1BeaconUrl: string             // Must be valid URL
  seedPhrase: string[]            // length === 12, each word required
  awsAccessKey: string            // Required if infraProvider === "aws"
  awsSecretKey: string            // Required if infraProvider === "aws"
  awsRegion: string               // Required if infraProvider === "aws"
  feeToken: enum(["TON", "ETH", "USDT", "USDC"])
  reuseDeployment: boolean        // Mainnet only
})
  .superRefine((data, ctx) => {
    // AWS credentials required if infraProvider === "aws"
    // local + Mainnet = invalid combo
  })
```

**Form Fields** (11 fields total):

| # | Field | Type | Validation | Notes |
|---|-------|------|-----------|-------|
| 1 | chainName | string | `/^[a-z0-9-]{3,32}$/` | 3-32 chars, lowercase/digits/hyphens |
| 2 | network | enum | ["Testnet", "Mainnet"] | Affects reuseDeployment availability |
| 3 | infraProvider | enum | ["aws", "local"] | Determines AWS field visibility |
| 4 | l1RpcUrl | string (URL) | Valid URL format | e.g., "https://1rpc.com/sepolia" |
| 5 | l1BeaconUrl | string (URL) | Valid URL format | e.g., "https://beacon.sepolia.example" |
| 6 | seedPhrase | string[] | length === 12 | Each word required, space-joined before API |
| 7 | awsAccessKey | string | Required if aws | Empty string if local provider |
| 8 | awsSecretKey | string | Required if aws | Empty string if local provider |
| 9 | awsRegion | string | Required if aws | Empty string if local provider |
| 10 | feeToken | enum | ["TON", "ETH", "USDT", "USDC"] | Fee payment token for deployment |
| 11 | reuseDeployment | boolean | Mainnet only | Reuse L1 deployment if true |

**Form Entry Points** (Lines 147-148 in usePresetWizard.ts):

```typescript
// usePresetWizard.ts line 139
const isValid = await form.trigger("presetBasicInfo");
if (!isValid) return;  // Exit if validation fails
updateCurrentStep(3);  // Proceed to Step 3 if valid
```

**Button Locations**:
- Previous button: page.tsx line 107-114
- Next button: page.tsx line 116-127
  - Condition: `disabled={currentStep === 1 && !selectedPresetId}`
  - Text: "Next" (step 2) or "Deploy Rollup" (step 3) with icon

---

## Step 3: goToNextStep() (Conditional Dispatch) → handleDeploy()

**Location**: usePresetWizard.ts lines 128-148

**Execution Flow**:

```typescript
goToNextStep() {
  // Line 129-136: Step 1 → Step 2
  if (state.currentStep === 1) {
    // Validate preset selection
    if (!selectedPresetId) return;
    updateCurrentStep(2);
    return;
  }

  // Line 138-143: Step 2 → Step 3
  if (state.currentStep === 2) {
    const isValid = await form.trigger("presetBasicInfo");  // Line 139
    if (!isValid) return;
    updateCurrentStep(3);
    return;
  }

  // Line 145-147: Step 3 → Deploy
  if (state.currentStep === 3) {
    await handleDeploy();  // ← CONDITIONAL DISPATCH (line 146)
  }
}
```

**Key Point**: `handleDeploy()` is called **conditionally** only when `currentStep === 3`. This ensures:
- Form validation passes (line 139)
- User confirms configuration (Step 3 review)
- Only then deployment API is called

---

## handleDeploy() Implementation (Lines 87-126 in usePresetWizard.ts)

**API Call Chain**:

1. **Get form values** (line 88):
   ```typescript
   const basicInfo = form.getValues("presetBasicInfo");
   ```

2. **Construct request object** (lines 94-111):
   ```typescript
   {
     presetId: selectedPresetId,
     chainName: basicInfo.chainName,
     network: basicInfo.network,
     infraProvider: basicInfo.infraProvider,
     seedPhrase: basicInfo.seedPhrase.join(" "),  // ← Convert array to string
     feeToken: basicInfo.feeToken,
     awsAccessKey: basicInfo.awsAccessKey ?? "",
     awsSecretKey: basicInfo.awsSecretKey ?? "",
     awsRegion: basicInfo.awsRegion ?? "",
     l1RpcUrl: basicInfo.l1RpcUrl,
     l1BeaconUrl: basicInfo.l1BeaconUrl,
     reuseDeployment: basicInfo.network === "Mainnet" ? (basicInfo.reuseDeployment ?? true) : undefined,
     overrides: overrides.length > 0 ? overrides : undefined,
   }
   ```

3. **Call API** (line 94):
   ```typescript
   const result = await startPresetDeployment(request);
   ```
   - Defined in presetService.ts (lines 45-53)
   - Calls `apiPost("stacks/thanos/preset-deploy", request)`

4. **Success handling** (lines 113-116):
   ```typescript
   toast.success("Deployment initiated!", { id: "preset-deploy" });
   setPendingDeploymentId(result.deploymentId);
   resetState();
   router.push("/rollup");
   ```

5. **Error handling** (lines 117-125):
   ```typescript
   catch (error) {
     const message = error.message || "Failed to initiate deployment";
     toast.error(message, { id: "preset-deploy" });
   }
   ```

---

## Request Interceptor (api.ts, Lines 26-41)

**Purpose**: Inject authorization headers before each API call

**Implementation**:

```typescript
apiClient.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem("accessToken");
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;  // Line 34
    }
    return config;
  },
  (error) => Promise.reject(error)
);
```

**Behavior**:
- Retrieves JWT token from `localStorage.accessToken`
- Sets `Authorization: Bearer <token>` header
- If no token present, request proceeds without authorization header

---

## Response Interceptor (api.ts, Lines 44-64)

**Purpose**: Handle response status codes and errors

**Behavior**:

| Status Code | Action |
|-------------|--------|
| 200-299 | Pass through to application |
| 401 | Remove `accessToken` from localStorage, reject promise |
| 403 | Log "Access denied" to console, reject promise |
| 4xx-5xx | Reject promise with error details |

**Implementation**:

```typescript
apiClient.interceptors.response.use(
  (response) => response,  // Line 45-46: Pass 200-299 through
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      localStorage.removeItem("accessToken");  // Line 53
    }
    if (error.response?.status === 403) {
      console.error("Access denied");  // Line 59
    }
    return Promise.reject(error);  // Line 62
  }
);
```

**Response Unwrapping** (api.ts, line 132):

The `apiPost()` helper returns `response.data`, which unwraps the Axios response:

```typescript
export const apiPost = async <T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<ApiResponse<T>> => {
  const response = await apiClient.post(url, data, config);
  return response.data;  // Line 132: Unwrap to ApiResponse<T>
};
```

---

## Backend Handler (presets.go, Lines 78-104)

**Endpoint**: `POST /stacks/thanos/preset-deploy`

**Handler Signature**:

```go
func (h *ThanosDeploymentHandler) PresetDeploy(c *gin.Context) {
  // Line 79-86: Deserialization
  var request dtos.PresetDeployRequest
  if err := c.ShouldBindJSON(&request); err != nil {
    c.JSON(http.StatusBadRequest, &entities.Response{...})
    return
  }

  // Line 88-96: Validate preset exists
  presetSvc := presets.NewService()
  if _, err := presetSvc.GetByID(request.PresetID); err != nil {
    c.JSON(http.StatusBadRequest, &entities.Response{...})
    return
  }

  // Line 98: Call service
  response, err := h.ThanosDeploymentService.CreateThanosStackFromPreset(c.Request.Context(), request)
  
  // ⚠️ KNOWN ISSUE (Line 99-101): Error logged but not returned to client
  if err != nil {
    logger.Error("failed to deploy preset stack", zap.Error(err))
    // BUG: Missing response.WriteError() or c.JSON() call
  }

  // Line 103: Return response (may be nil or error state)
  c.JSON(int(response.Status), response)
}
```

---

## ⚠️ Known Issue: Backend Error Response Missing (Issue #4)

**File**: `/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/presets.go`  
**Lines**: 99-101

**Problem**:

When `CreateThanosStackFromPreset()` fails, the error is logged to console (line 100) but:

1. No HTTP response is written to client
2. No error response structure is returned
3. Frontend receives undefined/null `response` object
4. Line 103 attempts to write `response.Status` (panic or 0)

**Current Code**:

```go
response, err := h.ThanosDeploymentService.CreateThanosStackFromPreset(c.Request.Context(), request)
if err != nil {
  logger.Error("failed to deploy preset stack", zap.Error(err))
  // MISSING: c.JSON(http.StatusInternalServerError, ...)
}
c.JSON(int(response.Status), response)  // response could be nil
```

**Impact on Frontend**:

- `toast.error(message)` receives incorrect/empty message
- User sees generic "Failed to initiate deployment" instead of specific error
- No validation details are communicated (e.g., "AWS credentials invalid")

**Proposed Fix**:

```go
if err != nil {
  logger.Error("failed to deploy preset stack", zap.Error(err))
  c.JSON(http.StatusInternalServerError, &entities.Response{
    Status:  http.StatusInternalServerError,
    Message: err.Error(),
    Data:    nil,
  })
  return  // ← Return early to avoid nil dereference
}
```

---

## Backend DTO Validation (ValidateProvider)

**Service**: `preset_deploy.go` (lines not specified in spec)

**Validation Rules**:

```
chainName:
  - Type: string
  - Regex: ^[a-z0-9-]{3,32}$
  - Constraint: 3-32 characters, lowercase, alphanumeric + hyphens

network:
  - Type: enum
  - Values: ["Testnet", "Mainnet"]
  - Constraint: Case-sensitive

infraProvider:
  - Type: enum
  - Values: ["aws", "local"]
  - Constraint: Determines AWS field validation

seedPhrase:
  - Type: string (space-separated in API, string[] in form)
  - Constraint: Must represent exactly 12 BIP39 words
  - Binding: Validates during mnemonic creation

awsAccessKey, awsSecretKey, awsRegion:
  - Type: string
  - Constraint: Required if infraProvider === "aws"
  - Validation: aws.ValidateCredentials() or similar

feeToken:
  - Type: enum
  - Values: ["TON", "ETH", "USDT", "USDC"]
  - Constraint: Must match preset's availableFeeTokens list

reuseDeployment:
  - Type: boolean
  - Constraint: Mainnet only, defaults to true
```

---

## Error Handling Scenarios

### Scenario 1: Preset Not Selected

**Trigger**: User clicks "Next" on Step 1 without selecting preset

**Frontend**:
- `goToNextStep()` line 130-132
- Check fails: `if (!selectedPresetId) { toast.error("Please select a preset..."); return; }`
- No API call made

**Result**: Toast error, stay on Step 1

---

### Scenario 2: Invalid Chain Name (Frontend)

**Trigger**: User enters chain name "123" (all digits, fails regex)

**Frontend**:
- `goToNextStep()` line 139: `form.trigger("presetBasicInfo")`
- Zod validation fails on `chainName` field (regex mismatch)
- Form returns error object

**Result**: Error message displayed on chainName field, stay on Step 2

---

### Scenario 3: Seed Phrase Word Count Mismatch

**Trigger**: User enters only 11 words instead of 12

**Frontend**:
- `goToNextStep()` line 139: `form.trigger("presetBasicInfo")`
- Zod validation fails on `seedPhrase.length(12)` (line 207 in create-rollup.ts)
- Zod constraint error: "Seed phrase must contain exactly 12 words"

**Result**: Error message on seedPhrase field, stay on Step 2

---

### Scenario 4: AWS Selected but Missing Credentials

**Trigger**: User selects `infraProvider: "aws"` but leaves AWS fields empty

**Frontend**:
- `goToNextStep()` line 139: `form.trigger("presetBasicInfo")`
- `superRefine()` adds issues for awsAccessKey, awsSecretKey, awsRegion (lines 215-225)
- Zod validation fails

**Result**: Error messages on each AWS field, stay on Step 2

---

### Scenario 5: Local Provider + Mainnet (Unsupported Combo)

**Trigger**: User selects `infraProvider: "local"` AND `network: "Mainnet"`

**Frontend**:
- `goToNextStep()` line 139: `form.trigger("presetBasicInfo")`
- `superRefine()` check fails (lines 226-228)
- Zod error: "Local deployment is not supported for Mainnet"

**Result**: Error message on network field, stay on Step 2

---

### Scenario 6: Invalid L1 RPC URL (Not a Valid URL)

**Trigger**: User enters `l1RpcUrl: "not a url"`

**Frontend**:
- `goToNextStep()` line 139: `form.trigger("presetBasicInfo")`
- Zod `.url()` validator fails (line 205)
- Error: "Must be a valid URL"

**Result**: Error displayed, stay on Step 2

---

### Scenario 7: infraProvider Validation Fails (AWS Credentials Invalid) - NEW

**Trigger**: User selects AWS provider but credentials don't authenticate

**Frontend**:
- Passes Step 2 validation (all fields populated)
- Calls `handleDeploy()` → `startPresetDeployment(request)`

**Backend** (hypothetical, depends on ValidateProvider() implementation):
- `ShouldBindJSON()` succeeds
- `ValidateProvider()` checks AWS credentials
- AWS SDK fails: "InvalidParameterValue: Invalid credentials"
- Returns 400 with error message

**Result**: Backend returns 400, frontend catches error in handleDeploy() line 117, shows toast

---

### Scenario 8: Seed Phrase Format Error (Invalid BIP39 Words) - NEW

**Trigger**: User enters 12 valid English words but not valid BIP39 mnemonic

**Frontend**:
- Passes Step 2 Zod validation (12 words entered)
- Calls `handleDeploy()` with `seedPhrase: "word1 word2 ... word12"`

**Backend**:
- Request binding succeeds
- Seed phrase is used to derive accounts
- BIP39 mnemonic validation fails: "Invalid seed phrase"
- CreateThanosStackFromPreset() returns error

**Result**: Backend returns 500 (BUG: not returned, see Issue #4), frontend shows generic error

---

### Scenario 9: Mainnet + Local Provider Combo (Backend Validation) - NEW

**Trigger**: Frontend superRefine() bypassed OR API called with invalid combo

**Frontend** (if bypassed):
- Tries to call API with `network: "Mainnet"` and `infraProvider: "local"`

**Backend**:
- `ValidateProvider()` rejects combo
- Returns 400: "Local deployment not supported for Mainnet"

**Result**: Frontend shows error toast, user returns to Step 2

---

## Validation Comparison: Frontend vs Backend

| Field | Frontend (Zod) | Backend (Go) | Difference |
|-------|---|---|---|
| chainName | Regex `/^[a-z0-9-]{3,32}$/` | Type validation in DTO | Frontend stricter (regex) |
| network | Enum check | Enum in DTO | Same |
| infraProvider | Enum check, combo validation | Provider type validation | Frontend prevents local+mainnet |
| seedPhrase | Array length === 12 | BIP39 mnemonic validation | Backend validates crypto |
| awsAccessKey | Required if aws | AWS SDK validation | Backend authenticates |
| feeToken | Enum check | Enum validation in DTO | Same |
| reuseDeployment | Type check | Optional in DTO | Frontend enforces mainnet-only |

**Defense-in-Depth Pattern**:
- Frontend validation provides quick user feedback (no API round-trip)
- Backend validation provides security (prevents bypasses via API)

---

## Toast Notification Lifecycle

**Loading State** (line 91):
```typescript
toast.loading("Initiating deployment...", { id: "preset-deploy" });
```
- Displays spinner with message
- Persists until dismissed or replaced

**Success State** (line 113):
```typescript
toast.success("Deployment initiated!", { id: "preset-deploy" });
```
- Replaces loading toast
- Shows for ~3 seconds then auto-dismisses
- Also updates context with deploymentId

**Error State** (line 124):
```typescript
toast.error(message, { id: "preset-deploy" });
```
- Replaces loading toast
- Persists until user dismisses
- Message from backend error or catch block

---

## Sequence Diagram (Detailed)

```
User              Frontend          API Proxy       Backend
 │                   │                  │              │
 ├──(Step 1)─Select Preset─────────────────────────────┤
 │                   │                                  │
 ├──(Step 2)─Fill Form──────────────────────────────────┤
 │                   │                                  │
 ├──(Step 3)─Click Next─────────────────────────────────┤
 │         goToNextStep() called                        │
 │                   │                                  │
 │        form.trigger("presetBasicInfo")              │
 │         [Zod validation in-browser]                 │
 │                   │                                  │
 │        await handleDeploy() [if valid]              │
 │         toast.loading("Initiating...")              │
 │                   │                                  │
 │        Construct request object                      │
 │         [seedPhrase.join(" ")]                       │
 │                   │                                  │
 │        apiPost("stacks/thanos/preset-deploy")       │
 │                   ├─Request Interceptor──────────────┤
 │                   │ [Add Authorization header]      │
 │                   ├──POST /api/proxy/...───────────>│
 │                   │        [Rewrite to :8000]       │
 │                   │                 ├─ShouldBindJSON()
 │                   │                 ├─GetByID(presetID)
 │                   │                 ├─CreateThanosStackFromPreset()
 │                   │ ⚠️ Error Handling Gap (BUG)      │
 │                   │ ✅ Success Path OK               │
 │                   │                 │                │
 │                   │<─HTTP 200/400/500────────────────┤
 │                   ├─Response Interceptor─────────────┤
 │                   │ [Check status code]              │
 │                   │                   │              │
 │         catch(error) OR response.data │              │
 │                   │                   │              │
 │         [Success] setPendingDeploymentId(id)         │
 │              router.push("/rollup")                  │
 │              toast.success("Deployment initiated!")  │
 │                   │                                  │
 │         [Error] toast.error(message)                 │
 │              Stay on Step 3                          │
 │                   │                                  │
```

---

## Key Characteristics

### 1. Preset-Based Simplification

**Classic Mode** (not detailed here):
- 4 steps: Network → Account & AWS → DAO → Review
- ~20 form fields

**Preset Mode** (this spec):
- 3 steps: Preset → Basic Info → Review & Deploy
- 11 form fields
- Preset defines: module enablement, Helm values, available fee tokens

### 2. Seedphrase Handling

**Form Storage**: Array of 12 strings for UX (per-word input)

```typescript
seedPhrase: Array(12).fill("")  // usePresetWizard.ts line 58
```

**API Transmission**: Space-separated single string

```typescript
seedPhrase: basicInfo.seedPhrase.join(" ")  // usePresetWizard.ts line 102
```

**Reason**: Backend SDK expects mnemonic string, frontend UX benefits from per-word editing.

### 3. AWS Provider Conditional Visibility

**When infraProvider === "local"**:
- AWS fields remain in form object
- Values: empty strings ("") or undefined
- Not sent to backend if local provider (optimization possible, see Issue #11)

**When infraProvider === "aws"**:
- AWS fields required
- Zod superRefine() enforces presence
- Values sent to backend for authentication

### 4. Network-Dependent Fields

**reuseDeployment**:
- Testnet: undefined (not applicable)
- Mainnet: boolean, defaults to true

```typescript
reuseDeployment: basicInfo.network === "Mainnet" ? (basicInfo.reuseDeployment ?? true) : undefined,
// usePresetWizard.ts line 109
```

**Impact**: Mainnet deployments can reuse L1 contract if already initialized.

---

## Summary of 11 Issues Fixed

| # | Issue | Category | Status |
|---|-------|----------|--------|
| 1 | Frontend entry point line numbers | Documentation | ✅ Fixed |
| 2 | Form fields list expanded (6→11) | Documentation | ✅ Fixed |
| 3 | handleDeploy call chain clarified | Documentation | ✅ Fixed |
| 4 | Backend error response missing | Known Bug | ✅ Documented |
| 5 | Validation logic detailed | Documentation | ✅ Fixed |
| 6 | Sequence diagram subdivided | Documentation | ✅ Fixed |
| 7 | Error scenarios expanded (6→9) | Documentation | ✅ Fixed |
| 8 | Zod schema validation rules explicit | Documentation | ✅ Fixed |
| 9 | Trigger function naming clarified | Documentation | ✅ Fixed |
| 10 | Response interceptor details added | Documentation | ✅ Fixed |
| 11 | Local provider field optimization suggested | Optimization | ✅ Documented |

---

## Recommendations

1. **Issue #4 Priority**: Fix backend error response in `presets.go` line 100
   - Add proper `c.JSON()` call on error
   - Add return statement to prevent nil dereference

2. **Issue #11 Improvement**: Optimize request payload for local provider
   - Exclude AWS fields from request when `infraProvider === "local"`
   - Reduces API payload size
   - Cleaner backend handling

3. **Future Enhancement**: Add request/response logging middleware
   - Log all preset deployments with timestamps
   - Audit trail for compliance

4. **Monitoring**: Add metrics for:
   - Form validation failure rates by field
   - Deployment success/failure rates by preset
   - Error message frequency

---

## References

- **Frontend**: `/Users/theo/workspace_tokamak/trh-platform-ui/src/app/rollup/create/page.tsx`
- **usePresetWizard Hook**: `/Users/theo/workspace_tokamak/trh-platform-ui/src/features/rollup/hooks/usePresetWizard.ts`
- **Zod Schemas**: `/Users/theo/workspace_tokamak/trh-platform-ui/src/features/rollup/schemas/create-rollup.ts`
- **API Layer**: `/Users/theo/workspace_tokamak/trh-platform-ui/src/lib/api.ts`
- **Preset Service**: `/Users/theo/workspace_tokamak/trh-platform-ui/src/features/rollup/services/presetService.ts`
- **Backend Handler**: `/Users/theo/workspace_tokamak/trh-backend/pkg/api/handlers/thanos/presets.go`

---

**Document Status**: Complete with all 11 issues addressed and documented.
