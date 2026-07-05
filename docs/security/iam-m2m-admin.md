# IAM Casdoor M2M Admin Boundary

IAM is the only platform service that should hold the elevated Casdoor management client.
Gateway must not hold this credential.

## Responsibility split

```text
External user token
  -> Gateway verifies OIDC/JWKS
  -> Gateway injects trusted principal headers and the internal boundary token
  -> IAM restores the user principal in gateway_trusted mode
  -> IAM performs accessx / SpiceDB authorization
  -> IAM uses its dedicated Casdoor admin M2M client for Casdoor management APIs
```

## Rules

1. Casdoor is used for authentication, OIDC token issuance, profile/user source, and management APIs.
2. Casdoor authorization is not used for business authorization.
3. Business authorization is owned by IAM + Kernel accessx + SpiceDB.
4. Gateway only verifies external tokens and forwards trusted identity. It does not get elevated Casdoor management credentials.
5. IAM must use a dedicated Casdoor service application for management calls such as user, organization, application, and group provisioning.
6. The dedicated admin client must be configured under `security.authn.casdoor.admin`.

## Required IAM configuration shape

```yaml
security:
  authn:
    mode: gateway_trusted
    provider: casdoor
    casdoor:
      # Browser/OIDC client used for login, callback, refresh, logout, and token verification config.
      organization_name: aisphere
      application_name: aisphere
      client_id: ${CASDOOR_LOGIN_CLIENT_ID}
      client_secret: ${CASDOOR_LOGIN_CLIENT_SECRET}

      # Elevated service application used only by IAM for Casdoor management APIs.
      admin:
        enabled: true
        organization_name: aisphere
        application_name: iam-service
        client_id: ${CASDOOR_IAM_M2M_CLIENT_ID}
        client_secret: ${CASDOOR_IAM_M2M_CLIENT_SECRET}
```

## Runtime policy

Before IAM performs a Casdoor management action, the caller must already be authenticated and authorized by IAM's own authorization path. The M2M admin credential is an implementation detail of IAM's adapter to Casdoor. It is not the authorization decision source.

```text
caller principal
  -> accessx.Check
  -> SpiceDB permission decision
  -> Casdoor admin M2M call only after allow
```
