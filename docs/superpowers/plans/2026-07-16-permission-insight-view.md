# Permission Insight View Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a unified IAM permission view that lets operators inspect a user, group, or resource without thinking in raw SpiceDB tuples.

**Architecture:** Phase 1 is frontend-first and reuses the current `AccessQueryService`, directory APIs, authorization admin relationship API, and projection drift APIs. Phase 2 adds a proto-first backend query API for directory projection events so the UI can show recent DTM/SpiceDB projection status by object.

**Tech Stack:** Next.js 16, React 19, TanStack Query, Vitest, aisphere-iam proto/gRPC/HTTP generation, Go service layer, SpiceDB relationships, DTM-backed directory projection events.

---

## File Map

- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/lib/api/types.ts`
  Add `permission-insight` tab type and optional projection event types after the backend phase.
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/layout/app-shell.tsx`
  Register the new tab and make it a valid URL target.
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/layout/sidebar.tsx`
  Add the new "权限视图" navigation entry near existing permission entries.
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/app/page.tsx`
  Route `permission-insight` to the new page.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`
  Owns object selection and orchestrates user, group, and resource views.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/object-search.tsx`
  Searches users, groups, and resources from existing hooks.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/entitlement-list.tsx`
  Shared entitlement list with source labels and expandable permission detail.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/group-permission-panel.tsx`
  Shows a group as both resource and subject.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/relationship-details.tsx`
  Collapsible raw SpiceDB tuple view for selected object.
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/pages/organization-workspace.tsx`
  Add an optional `permissions` tab slot.
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/pages/groups-page.tsx`
  Add "查看权限" entry points for selected group/user and optionally embed a lightweight permission tab.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.test.tsx`
  Tests route-level behavior and group dual-perspective rendering.
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/entitlement-list.test.tsx`
  Tests source labels and source path rendering.
- Modify later: `E:/coding/aisphereio/aisphere-iam/api/iam/v1/iam.proto`
  Add projection event list API in Phase 2.
- Modify later: `E:/coding/aisphereio/aisphere-iam/internal/data/identity_mode.go`
  Add read-only list helper for `iam_directory_projection_events`.
- Modify later: `E:/coding/aisphereio/aisphere-iam/internal/service/directory_projection.go`
  Expose projection event list API.
- Create later: `E:/coding/aisphereio/aisphere-iam/internal/service/directory_projection_events_test.go`
  Backend tests for filtering projection events.

## Phase 1: Unified Frontend View With Existing APIs

### Task 1: Baseline And Encoding Check

**Files:**
- Inspect: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/*.tsx`
- Inspect: `E:/coding/aisphereio/aisphere-iam-front/src/components/pages/groups-page.tsx`
- Inspect: `E:/coding/aisphereio/aisphere-iam-front/src/lib/authz/schema-summary.ts`

- [ ] **Step 1: Check current frontend health**

Run:

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
npm run test:run -- src/components/access-control/access-control-page.test.tsx
```

Expected: Typecheck and targeted tests pass before feature work. If typecheck fails only because of existing unrelated files, record the exact failure and keep feature tests targeted.

- [ ] **Step 2: Search for mojibake in permission UI files**

Run:

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
rg -n "鈹|鎺|鏉|缁|璧|鐢|鍔|閫|鏌|鍙|涓|骞|椤|绠" src/components/access-control src/components/pages src/lib/authz src/app/page.tsx src/components/layout
```

Expected: Existing Chinese mojibake is visible in several files. Fix only files touched by this feature in later tasks; avoid broad rewrite unless you choose to do a dedicated cleanup commit.

### Task 2: Add A Stable Permission Insight Tab

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/lib/api/types.ts`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/layout/app-shell.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/layout/sidebar.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/app/page.tsx`
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`

- [ ] **Step 1: Write the failing route test**

Create or extend a page routing test if one already exists. The assertion is that `permission-insight` is accepted as a tab and renders a page title.

```tsx
import { render, screen } from '@testing-library/react';
import { PermissionInsightPage } from './permission-insight-page';

describe('PermissionInsightPage', () => {
  it('renders the unified permission view shell', () => {
    render(<PermissionInsightPage identityOrg="aisphere" />);
    expect(screen.getByRole('heading', { name: '权限视图' })).toBeInTheDocument();
    expect(screen.getByText('搜索人员、组织或资源')).toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the failing test**

Run:

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/permission-insight-page.test.tsx
```

Expected: FAIL because `PermissionInsightPage` does not exist.

- [ ] **Step 3: Add the minimal page shell**

Create `permission-insight-page.tsx` with:

```tsx
'use client';

import { Search } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';

export function PermissionInsightPage({ identityOrg }: { identityOrg: string }) {
  return (
    <section className="space-y-4 p-6">
      <div>
        <h2 className="text-xl font-semibold">权限视图</h2>
        <p className="text-sm text-muted-foreground">
          搜索人员、组织或资源，查看当前有效权限、来源路径和技术关系。
        </p>
      </div>
      <Card>
        <CardContent className="p-4">
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input className="pl-8" placeholder="搜索人员、组织或资源" aria-label="搜索人员、组织或资源" />
          </div>
          <div className="mt-4 rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
            在左侧搜索并选择对象后，这里会展示权限结果。
          </div>
        </CardContent>
      </Card>
      <span className="sr-only">{identityOrg}</span>
    </section>
  );
}
```

- [ ] **Step 4: Wire the tab**

Update `Tab` in `src/lib/api/types.ts` to include:

```ts
| 'permission-insight'
```

Add `permission-insight` to `validTabs` in `app-shell.tsx`.

In `page.tsx`, import and route:

```tsx
import { PermissionInsightPage } from '@/components/access-control/permission-insight/permission-insight-page';

if (tab === 'permission-insight') return <PermissionInsightPage identityOrg={identityOrg} />;
```

Add a sidebar entry near permission pages with a `SearchCheck` or `Route` lucide icon and label `权限视图`.

- [ ] **Step 5: Verify**

Run:

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/permission-insight-page.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add src/lib/api/types.ts src/components/layout/app-shell.tsx src/components/layout/sidebar.tsx src/app/page.tsx src/components/access-control/permission-insight
git commit -m "feat: add IAM permission insight entry"
```

### Task 3: Build Shared Entitlement Rendering

**Files:**
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/entitlement-list.tsx`
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/entitlement-list.test.tsx`

- [ ] **Step 1: Write source-label tests**

```tsx
import { render, screen } from '@testing-library/react';
import { EntitlementList } from './entitlement-list';
import type { IamEntitlement } from '@/lib/api/types';

const base: IamEntitlement = {
  id: 'e1',
  subject: { type: 'user', id: 'alice' },
  resource: { type: 'project', id: 'project-a' },
  roleKey: 'owner',
  permissions: ['view', 'manage'],
  sourceType: 'GROUP_GRANT',
  sourceSubject: { type: 'group', id: 'platform' },
};

describe('EntitlementList', () => {
  it('shows group source path without raw SpiceDB syntax by default', () => {
    render(<EntitlementList entitlements={[base]} emptyText="empty" />);
    expect(screen.getByText('project-a')).toBeInTheDocument();
    expect(screen.getByText('owner')).toBeInTheDocument();
    expect(screen.getByText('通过组织 platform')).toBeInTheDocument();
    expect(screen.queryByText(/group:platform#member/)).not.toBeInTheDocument();
  });
});
```

- [ ] **Step 2: Run the failing test**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/entitlement-list.test.tsx
```

Expected: FAIL because component does not exist.

- [ ] **Step 3: Implement the component**

Implement compact rows with these source labels:

```ts
function sourceText(entitlement: IamEntitlement): string {
  if (entitlement.sourceType === 'DIRECT_GRANT') return '直接授权';
  if (entitlement.sourceType === 'GROUP_GRANT') return `通过组织 ${entitlement.sourceSubject?.id || '-'}`;
  if (entitlement.sourceType === 'PARENT_INHERITANCE') return `继承自父资源 ${entitlement.sourceResource?.id || '-'}`;
  if (entitlement.sourceType === 'ORG_INHERITANCE') return `继承自身份域 ${entitlement.sourceResource?.id || '-'}`;
  if (entitlement.sourceType === 'PLATFORM_INHERITANCE') return `继承自平台 ${entitlement.sourceResource?.id || '-'}`;
  return '未知来源';
}
```

Rows should show:

- resource type and id
- subject id if the list is resource-oriented
- role key
- source text
- permission count and expanded permission badges
- revoke button only when a callback is passed and `revocableHere && grantId`

- [ ] **Step 4: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/entitlement-list.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add src/components/access-control/permission-insight/entitlement-list.tsx src/components/access-control/permission-insight/entitlement-list.test.tsx
git commit -m "feat: add shared entitlement list"
```

### Task 4: Add Object Search For Users, Groups, And Resources

**Files:**
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/object-search.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`

- [ ] **Step 1: Define a local selected-object type**

Use this type in `object-search.tsx` and export it:

```ts
export type PermissionInsightObject =
  | { kind: 'user'; id: string; label: string; subtitle?: string }
  | { kind: 'group'; id: string; label: string; subtitle?: string }
  | { kind: 'resource'; resourceType: string; id: string; label: string; subtitle?: string };
```

- [ ] **Step 2: Implement search from existing hooks**

Use:

- `useIamExternalUsers(identityOrg, { pageSize: 500 })`
- `useIamDirectoryGroups(identityOrg)`
- `useIamResources(identityOrg)`

Filter client-side by `label`, `id`, `email`, `username`, and `resourceType`. Render three sections: `人员`, `组织`, `资源`. Keep the list dense and operational, not card-heavy.

- [ ] **Step 3: Wire selection into the page**

`PermissionInsightPage` should hold:

```ts
const [selected, setSelected] = useState<PermissionInsightObject | null>(null);
```

Render selected object header:

```tsx
{selected ? (
  <div className="rounded-lg border bg-card px-3 py-2">
    <div className="text-sm font-medium">{selected.label}</div>
    <div className="text-xs text-muted-foreground">{selected.subtitle || selected.id}</div>
  </div>
) : null}
```

- [ ] **Step 4: Verify manually**

Run:

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run dev
```

Open `http://localhost:3001/?tab=permission-insight`, log in if needed, and search for a known user, group, and resource.

- [ ] **Step 5: Commit**

```powershell
git add src/components/access-control/permission-insight/object-search.tsx src/components/access-control/permission-insight/permission-insight-page.tsx
git commit -m "feat: search IAM permission insight objects"
```

### Task 5: Implement User And Resource Views

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`
- Use: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/entitlement-list.tsx`

- [ ] **Step 1: User view**

When `selected.kind === 'user'`, call:

```ts
const subjectEntitlements = useIamSubjectEntitlements(
  identityOrg,
  selected?.kind === 'user' ? { type: 'user', id: selected.id } : null,
);
const userGroups = useIamDirectoryGroups(
  identityOrg,
  selected?.kind === 'user' ? { userId: selected.id } : undefined,
);
```

Render tabs:

- `有效权限`: `EntitlementList` for all subject entitlements
- `所属组织`: group badges/path rows
- `技术关系`: relationship details filtered by `subject_type=user&subject_id={id}`

- [ ] **Step 2: Resource view**

When `selected.kind === 'resource'`, call:

```ts
const resourceAccess = useIamResourceAccess(
  identityOrg,
  selected?.kind === 'resource' ? { type: selected.resourceType, id: selected.id } : null,
);
```

Render tabs:

- `有权限的人和组织`: `EntitlementList` with subject column visible
- `直接授权`: filter `sourceType === 'DIRECT_GRANT'`
- `继承权限`: filter `sourceType !== 'DIRECT_GRANT'`
- `技术关系`: relationship details filtered by selected resource

- [ ] **Step 3: Empty and loading states**

Use exact empty text:

- user with no entitlements: `当前人员没有有效资源权限。`
- resource with no access: `当前资源没有有效授权。`
- loading: `正在加载权限数据...`
- error: show `error.message`

- [ ] **Step 4: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/entitlement-list.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add src/components/access-control/permission-insight/permission-insight-page.tsx
git commit -m "feat: show user and resource permission insight"
```

### Task 6: Implement Group Dual-Perspective View

**Files:**
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/group-permission-panel.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`
- Test: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.test.tsx`

- [ ] **Step 1: Write the group perspective test**

Test that the UI labels both meanings of group:

```tsx
it('explains group as both subject and resource', () => {
  render(<GroupPermissionPanel identityOrg="aisphere" groupId="platform" groupLabel="Platform" />);
  expect(screen.getByText('组织作为权限主体')).toBeInTheDocument();
  expect(screen.getByText('组织作为可管理资源')).toBeInTheDocument();
});
```

- [ ] **Step 2: Implement `GroupPermissionPanel`**

Call:

```ts
const groupSubjectEntitlements = useIamSubjectEntitlements(
  identityOrg,
  { type: 'group', id: groupId },
);

const groupResourceAccess = useIamResourceAccess(
  identityOrg,
  { type: 'group', id: groupId },
);

const members = useIamExternalUsers(identityOrg, { groupId, pageSize: 500 });
```

Render tabs:

- `组织作为权限主体`: this group has grants on resources; members may inherit them.
- `组织作为可管理资源`: who can manage this group, manage members, or manage permissions.
- `成员影响`: list current members and a note `成员会继承“组织作为权限主体”中的 group 授权。`
- `技术关系`: raw relationship details for both `resource=group:{id}` and `subject=group:{id}`.

- [ ] **Step 3: Add group case in page**

```tsx
{selected?.kind === 'group' ? (
  <GroupPermissionPanel identityOrg={identityOrg} groupId={selected.id} groupLabel={selected.label} />
) : null}
```

- [ ] **Step 4: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run test:run -- src/components/access-control/permission-insight/permission-insight-page.test.tsx
npm run typecheck
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add src/components/access-control/permission-insight/group-permission-panel.tsx src/components/access-control/permission-insight/permission-insight-page.tsx src/components/access-control/permission-insight/permission-insight-page.test.tsx
git commit -m "feat: explain group permission perspectives"
```

### Task 7: Add Raw Relationship Details As Collapsed Technical Context

**Files:**
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/relationship-details.tsx`

- [ ] **Step 1: Implement resource and subject filters**

Use `useIamAuthzRelationships` from `src/hooks/use-authz`. Support these props:

```ts
type RelationshipDetailsProps =
  | { mode: 'resource'; resourceType: string; resourceId: string }
  | { mode: 'subject'; subjectType: string; subjectId: string }
  | { mode: 'group'; groupId: string };
```

For `mode='group'`, run two queries:

- resource side: `{ resourceType: 'group', resourceId: groupId }`
- subject side: `{ subjectType: 'group', subjectId: groupId }`

- [ ] **Step 2: Keep raw tuples hidden by default**

Render a collapsed button label `显示 SpiceDB 原始关系`. After expansion, show tuples in monospaced rows:

```tsx
{resource.type}:{resource.id}#{relation}@{subject.type}:{subject.id}{subject.relation ? `#${subject.relation}` : ''}
```

- [ ] **Step 3: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add src/components/access-control/permission-insight/relationship-details.tsx src/components/access-control/permission-insight
git commit -m "feat: add permission insight relationship details"
```

### Task 8: Link Organization Page To Permission Insight

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/pages/organization-workspace.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/pages/groups-page.tsx`

- [ ] **Step 1: Add optional permission tab slot**

Update `OrganizationWorkspaceTabs` props:

```ts
permissions?: ReactNode;
```

If provided, render a fourth tab:

```tsx
<TabsTrigger value="permissions" className="min-w-24 rounded-lg px-3 text-xs">
  <Shield className="h-3.5 w-3.5" />
  权限
</TabsTrigger>
```

Use `Shield` from lucide-react.

- [ ] **Step 2: Add selected group permission tab**

In `groups-page.tsx`, when `selection.kind === 'group'`, pass:

```tsx
permissions={(
  <GroupPermissionPanel
    identityOrg={zoneId}
    groupId={groupID(selection.group)}
    groupLabel={groupLabel(selection.group)}
  />
)}
```

Import `GroupPermissionPanel`.

- [ ] **Step 3: Add selected user button**

When `selection.kind === 'user'`, add a button:

```tsx
<Button
  size="sm"
  variant="outline"
  onClick={() => {
    const url = new URL(window.location.href);
    url.searchParams.set('tab', 'permission-insight');
    url.searchParams.set('subject_type', 'user');
    url.searchParams.set('subject_id', userID(selection.user));
    window.history.replaceState({}, '', url.toString());
    window.dispatchEvent(new PopStateEvent('popstate'));
  }}
>
  <Shield className="mr-1 h-3.5 w-3.5" />
  查看权限
</Button>
```

Also teach `PermissionInsightPage` to read `subject_type=user&subject_id=...` on mount and preselect the object after users are loaded.

- [ ] **Step 4: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
npm run test:run
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add src/components/pages/organization-workspace.tsx src/components/pages/groups-page.tsx src/components/access-control/permission-insight
git commit -m "feat: connect organization page to permission insight"
```

### Task 9: Frontend Verification Pass

**Files:**
- Validate whole frontend feature.

- [ ] **Step 1: Static checks**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
npm run lint
npm run test:run
```

Expected: PASS.

- [ ] **Step 2: Manual smoke**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run dev
```

Manual cases:

- `http://localhost:3001/?tab=permission-insight` opens.
- Search a user and see effective permissions.
- Search a group and see both group perspectives.
- Search a resource and see who has access.
- From group page, selected group has a `权限` tab.
- Raw SpiceDB relationships are hidden until expanded.
- The page can be used without knowing `relation`, `subject_relation`, or tuple syntax.

- [ ] **Step 3: Commit any final UI fixes**

```powershell
git status --short
git add src
git commit -m "fix: polish permission insight UX"
```

Only run this commit if Step 2 required fixes.

## Phase 2: Backend Projection Event Query

### Task 10: Add Proto Contract For Projection Event Listing

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam/api/iam/v1/iam.proto`

- [ ] **Step 1: Add request/reply messages**

Add near existing directory projection messages:

```proto
message ListDirectoryProjectionEventsRequest {
  string org_id = 1 [(google.api.field_behavior) = REQUIRED, (buf.validate.field).string.min_len = 1];
  string aggregate_type = 2;
  string aggregate_id = 3;
  string status = 4;
  int32 page_size = 5;
  string page_token = 6;
}

message DirectoryProjectionEvent {
  string id = 1;
  string source = 2;
  string aggregate_type = 3;
  string aggregate_id = 4;
  string operation = 5;
  string status = 6;
  int32 retry_count = 7;
  string last_error = 8;
  google.protobuf.Timestamp next_run_at = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

message ListDirectoryProjectionEventsReply {
  repeated DirectoryProjectionEvent events = 1;
  string next_page_token = 2;
  int64 total_size = 3;
}
```

- [ ] **Step 2: Add service method**

Add under `IAMDirectoryProjectionService`:

```proto
rpc ListDirectoryProjectionEvents(ListDirectoryProjectionEventsRequest) returns (ListDirectoryProjectionEventsReply) {
  option (google.api.http) = { post: "/v1/iam/directory/projections:list-events" body: "*" };
  option (aisphere.access.v1.policy) = {
    exposure: AUTHORIZED
    authz: { action: "view_relationships" resource: "iam_authz:global" audience: "iam-service" mode: SELF_CHECK }
    audit: { enabled: true event: "iam.directory_projection.event_list" risk: "medium" }
  };
}
```

- [ ] **Step 3: Generate API**

```powershell
cd E:\coding\aisphereio\aisphere-iam
make api
make proto-check
```

Expected: generated Go/OpenAPI files update and proto checks pass.

- [ ] **Step 4: Commit**

```powershell
git add api docs/openapi deploy/generated
git commit -m "feat: add directory projection event query contract"
```

### Task 11: Implement Projection Event Repository Read

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam/internal/data/identity_mode.go`
- Test: `E:/coding/aisphereio/aisphere-iam/internal/data/identity_projection_event_test.go`

- [ ] **Step 1: Write repository test**

Test filtering by aggregate type/id and status using `IdentityProjectionEventModel` records. Expected behavior:

- returns newest first
- respects `page_size`
- filters by `aggregate_type`, `aggregate_id`, and `status`

- [ ] **Step 2: Add list options and method**

Add:

```go
type IdentityProjectionEventListOptions struct {
	AggregateType string
	AggregateID   string
	Status        string
	PageSize      int
	PageToken     string
}
```

Add method on `IdentityProjectionDispatcher`:

```go
func (d *IdentityProjectionDispatcher) ListEvents(ctx context.Context, opts IdentityProjectionEventListOptions) ([]IdentityProjectionEventModel, string, int64, error)
```

Implementation rules:

- default page size: 50
- max page size: 200
- page token is a decimal offset
- order by `created_at DESC`
- return `nextPageToken` when more rows exist

- [ ] **Step 3: Verify data tests**

```powershell
cd E:\coding\aisphereio\aisphere-iam
go test ./internal/data -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add internal/data/identity_mode.go internal/data/identity_projection_event_test.go
git commit -m "feat: list directory projection events"
```

### Task 12: Expose Projection Event Service Method

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam/internal/service/directory_projection.go`
- Create: `E:/coding/aisphereio/aisphere-iam/internal/service/directory_projection_events_test.go`

- [ ] **Step 1: Write service test**

Test that `ListDirectoryProjectionEvents` maps model fields to proto fields and rejects empty `org_id`.

- [ ] **Step 2: Implement method**

Add method on `IAMDirectoryProjectionService`:

```go
func (s *IAMDirectoryProjectionService) ListDirectoryProjectionEvents(ctx context.Context, req *v1.ListDirectoryProjectionEventsRequest) (*v1.ListDirectoryProjectionEventsReply, error)
```

Validation:

- trim `org_id`
- if empty, return `authn.ErrInvalidTokenRequest("org_id is required")`
- if `ops` or `Projection` is nil, return `authz.ErrBackendFailed("directory projection service is not configured", nil)`

Mapping:

- model `ID` -> proto `id`
- model `AggregateType` -> proto `aggregate_type`
- model `AggregateID` -> proto `aggregate_id`
- model `Status` -> proto `status`
- timestamps through `timestamppb.New`

- [ ] **Step 3: Verify service tests**

```powershell
cd E:\coding\aisphereio\aisphere-iam
go test ./internal/service -count=1
```

Expected: PASS except for any pre-existing unrelated failures. If unrelated failures exist, run the specific new test and record the package-level failure separately.

- [ ] **Step 4: Commit**

```powershell
git add internal/service/directory_projection.go internal/service/directory_projection_events_test.go
git commit -m "feat: expose directory projection event list"
```

### Task 13: Add Projection Event UI

**Files:**
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/lib/api/adapters/directory.ts`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/hooks/use-iam.ts`
- Create: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/projection-events-panel.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/permission-insight-page.tsx`
- Modify: `E:/coding/aisphereio/aisphere-iam-front/src/components/access-control/permission-insight/group-permission-panel.tsx`

- [ ] **Step 1: Sync frontend generated API**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run contract:sync
npm run api:generate
```

Expected: generated directory projection service includes `ListDirectoryProjectionEvents`.

- [ ] **Step 2: Add adapter and hook**

Adapter:

```ts
listProjectionEvents: (params: {
  orgId: string;
  aggregateType?: string;
  aggregateId?: string;
  status?: string;
  pageSize?: number;
  pageToken?: string;
}) => iAMDirectoryProjectionServiceListDirectoryProjectionEvents({
  orgId: params.orgId,
  aggregateType: params.aggregateType,
  aggregateId: params.aggregateId,
  status: params.status,
  pageSize: params.pageSize,
  pageToken: params.pageToken,
})
```

Hook:

```ts
export function useIamDirectoryProjectionEvents(orgId: string, params?: {
  aggregateType?: string;
  aggregateId?: string;
  status?: string;
  pageSize?: number;
}) {
  return useQuery({
    queryKey: ['iam', 'directory-projection-events', orgId, params],
    queryFn: () => iamDirectoryApi.listProjectionEvents({ orgId, ...params }),
    enabled: Boolean(orgId),
  });
}
```

- [ ] **Step 3: Render event status**

`ProjectionEventsPanel` props:

```ts
type ProjectionEventsPanelProps = {
  identityOrg: string;
  aggregateType?: string;
  aggregateId?: string;
};
```

Rows show:

- operation
- status badge
- retry count
- last error if present
- created/updated time

Use statuses: `pending`, `submitted`, `projecting`, `synced`, `failed`, `archived`.

- [ ] **Step 4: Wire into views**

For user: aggregate filters are not guaranteed by current event model, so show org-level recent events with a note `当前投影事件按目录对象聚合，用户成员变更通常表现为 group 事件。`

For group: use:

```tsx
<ProjectionEventsPanel identityOrg={identityOrg} aggregateType="group" aggregateId={groupId} />
```

For resource: show only if relevant aggregate mapping exists; otherwise keep it absent.

- [ ] **Step 5: Verify**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
npm run test:run
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add src/lib/api src/hooks/use-iam.ts src/components/access-control/permission-insight
git commit -m "feat: show directory projection events in permission insight"
```

## Final Verification

- [ ] **Backend targeted checks**

```powershell
cd E:\coding\aisphereio\aisphere-iam
make api
make proto-check
go test ./internal/data ./internal/service ./internal/server -count=1
```

Expected: proto checks pass. If `internal/server` has the known existing contract failure around `manage_members`, fix that in the same branch or document it as pre-existing before merging.

- [ ] **Frontend checks**

```powershell
cd E:\coding\aisphereio\aisphere-iam-front
npm run typecheck
npm run lint
npm run test:run
npm run build
```

Expected: PASS.

- [ ] **Manual acceptance**

Manual acceptance criteria:

- Operators can open `权限视图` from the sidebar.
- Searching a user shows all effective permissions and source paths.
- Searching a group shows both `组织作为权限主体` and `组织作为可管理资源`.
- Searching a resource shows all subjects with access.
- Raw SpiceDB tuples are present but hidden under technical details.
- From the organization page, a selected group can reach its permission view.
- After adding/removing a group member, refreshing the selected group/user view shows the latest effective permissions.
- After Phase 2, group projection events show latest DTM/SpiceDB projection status.

## Suggested Commit Sequence

1. `feat: add IAM permission insight entry`
2. `feat: add shared entitlement list`
3. `feat: search IAM permission insight objects`
4. `feat: show user and resource permission insight`
5. `feat: explain group permission perspectives`
6. `feat: add permission insight relationship details`
7. `feat: connect organization page to permission insight`
8. `feat: add directory projection event query contract`
9. `feat: list directory projection events`
10. `feat: expose directory projection event list`
11. `feat: show directory projection events in permission insight`

## Scope Notes

- Do not make users read raw SpiceDB tuple syntax for the normal flow.
- Do not replace `AccessQueryService` in Phase 1; it is already the correct aggregation layer.
- Do not make the projection event API mutate state; retry/reconcile/drift are separate admin actions.
- Do not broaden group move authorization in this plan. That belongs in a separate backend security fix unless you intentionally merge the efforts.
