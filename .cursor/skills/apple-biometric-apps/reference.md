# Apple biometric apps — reference

Sourced from Apple Developer Documentation (LocalAuthentication, Security) and WWDC22 10108 via the Apple docs MCP, 5 Sep 2026. HIG and App Review 2.5.13 are not in that catalog.

## LAPolicy

| Policy | Since | Behavior |
|--------|-------|----------|
| `deviceOwnerAuthentication` | iOS 9+ | Biometrics first, then system passcode. Login sample default. |
| `deviceOwnerAuthenticationWithBiometrics` | iOS 8+ | Biometrics only. Fallback button → `userFallback`. |
| `deviceOwnerAuthenticationWithBiometricsOrCompanion` | iOS 18+ / macOS 15+ | Biometry and nearby paired companion (Watch, Mac, Vision) in parallel; degrades if one side is unavailable. |
| `deviceOwnerAuthenticationWithCompanion` | iOS 18+ / macOS 15+ | Companion required. `companionNotAvailable` if none nearby. |
| `deviceOwnerAuthenticationWithWristDetection` | watchOS 9+ | Watch on wrist + passcode. Fails if removed. |

Deprecated: `deviceOwnerAuthenticationWithWatch`, `deviceOwnerAuthenticationWithBiometricsOrWatch`.

`LABiometryType`: `.faceID`, `.touchID`, `.opticID`, `.none` — meaningful only after `canEvaluatePolicy`.

## Access-control flags

Accessibility (when the item exists) is independent of authentication (who may use it). Prefer the most restrictive accessibility that still works (`kSecAttrAccessibleWhenPasscodeSetThisDeviceOnly` in Apple’s Keychain + Face ID sample).

| Flag | Effect |
|------|--------|
| `userPresence` | Biometrics, or device passcode fallback. Sample default. |
| `biometryAny` | Any enrolled Face ID / Touch ID. Survives add/remove/re-enroll. |
| `biometryCurrentSet` | Invalidated if fingers added/removed or Face ID re-enrolled. |
| `applicationPassword` | Extra app-specific password, not the device passcode. |
| `privateKeyUsage` | Required for SE private-key sign/decrypt. Without it, generation succeeds but signing fails. |

SE keys: A7+ / Touch ID Mac or Apple silicon. NIST P-256 only. Cannot import an existing private key. Combine `privateKeyUsage` with a biometry flag.

## LARight (iOS 16+)

Default right: Face ID, Touch ID, Apple Watch, or device passcode. Use `LAAuthenticationRequirement.biometryCurrentSet` or `.biometry(fallback: .devicePasscode)` to tighten.

`LARightStore.saveRight` persists an SE-backed key (optional immutable secret). Keep a strong reference while authorized. `deauthorize()` or deallocation drops the grant. Private-key ops require a live authorization; public-key ops do not.

WWDC22 pattern for existing SE keys:

1. `SecItemCopyMatching` with `kSecReturnAttributes` to get `kSecAttrAccessControl`
2. `context.evaluateAccessControl(acl, operation: .useKeySign, localizedReason:)`
3. `SecItemCopyMatching` with `kSecUseAuthenticationContext: context`
4. `SecKeyCreateSignature` — no second sheet until `invalidate()`

Do not assume a prior successful ACL evaluation guarantees the next one.

## Enrollment / companion state (iOS 18+)

- `LAContext.domainState.biometry.stateHash` — changes when fingers/faces are added or removed; does not describe how. Nil if nothing enrolled.
- `domainState.companion.stateHash` / `stateHash(for:)` — pairing set.
- Combined `domainState.stateHash`.
- `evaluatedPolicyDomainState` — same idea, deprecated iOS 18 / macOS 15 / visionOS 2.
- `LAEnvironment` + `Observer` — watch biometry, companion, and password mechanisms without prompting (watchOS 11+ too).

## Errors to handle

| Code | Response |
|------|----------|
| `canEvaluatePolicy` failure | Fall back to username/password or passkey. |
| `userCancel` / `systemCancel` / `appCancel` | Return; do not retry in a loop. |
| `userFallback` | Your PIN/password UI, or use `.deviceOwnerAuthentication`. |
| `biometryLockout` | Stop Face ID prompts; offer account credentials. Prefer this over deprecated `touchIDLockout`. |
| `companionNotAvailable` | Fall back to biometrics or account credentials. |
| `biometryNotPaired` / `biometryDisconnected` | Mac removable Touch ID accessory (macOS 11.2+). Fall back to password. |

`touchIDAuthenticationAllowableReuseDuration` only covers a recent **device unlock** with Touch ID (max ~5 minutes), not Keychain retrieval prompts.

## Review and copy

- **2.5.13** — LocalAuthentication for facial account auth, not ARKit; alternate method for under 13.
- **NSFaceIDUsageDescription** — required iOS 11+; missing key blocks Face ID.
- HIG: “Sign In with Face ID”, no in-app enable toggle, don’t say “passcode” for an app password, don’t mention Face ID on devices that lack it. Face ID starts scanning immediately — warn first.
- Prefer passkeys or Sign in with Apple for the **account**; biometrics unlock local access.

## Primary URLs

- https://developer.apple.com/documentation/localauthentication
- https://developer.apple.com/documentation/localauthentication/logging-a-user-into-your-app-with-face-id-or-touch-id
- https://developer.apple.com/documentation/localauthentication/accessing-keychain-items-with-face-id-or-touch-id
- https://developer.apple.com/documentation/localauthentication/laright
- https://developer.apple.com/documentation/localauthentication/lacontext/domainstate
- https://developer.apple.com/documentation/security/protecting-keys-with-the-secure-enclave
- https://developer.apple.com/documentation/security/restricting-keychain-item-accessibility
- https://developer.apple.com/videos/play/wwdc2022/10108/
- https://developer.apple.com/design/human-interface-guidelines/managing-accounts
- https://developer.apple.com/app-store/review/guidelines/
