# Identity Admin Local TODO

After local generation, verify the following items manually or with tests.

## Commands

```bash
make api
make proto-check
make test
make build
```

## Checks

- Generated HTTP binding exists for IAMIdentityAdminService.
- Generated Gateway module exists for IAMIdentityAdminService.
- Generated request info resolver exists for IAMIdentityAdminService.
- Generated access resolver exists for IAMIdentityAdminService.
- CreateUser, UpdateUser, DisableUser and DeleteUser require AuthZ.
- CreateGroup, UpdateGroup, DeleteGroup, AssignUserToGroup and RemoveUserFromGroup require AuthZ.
- casdoor_local allows user writes after AuthZ.
- external_oidc rejects upstream user writes but allows local application group writes after AuthZ.
- Group membership changes are projected into AuthZ relationships.
- The old plain HTTP /v1/users route stays unavailable.
