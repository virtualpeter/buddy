---
name: apple-biometric-apps
description: >-
  Applies Apple LocalAuthentication guidance for Face ID, Touch ID, and Optic ID
  in iOS/macOS/visionOS apps (Keychain ACLs, Secure Enclave keys, LARight,
  iOS 18 domain state and companion policies). Use when the user asks about
  biometric-secured apps, Face ID/Touch ID login, LocalAuthentication,
  evaluatePolicy vs Keychain, or App Review 2.5.13. Do not use for Jamf
  passcode/biometric configuration profiles or buddy builtins.
---

# Apple biometric-secured apps

This is **app** LocalAuthentication guidance, not Jamf MDM passcode or Touch ID *restriction* payloads. For profiles, use buddy profile-payload docs instead.

Read [reference.md](reference.md) for flags, policies, errors, and source URLs. A briefing canvas lives at [apple-biometric-app-guidance.canvas.tsx](/Users/peter/.cursor/projects/Users-peter-work-buddy/canvases/apple-biometric-app-guidance.canvas.tsx). Prefer the Apple docs MCP (`get_apple_doc_content`, `search_framework_symbols`) over web search; `search_apple_docs` may return empty.

## Non-negotiables

- Biometrics **supplement** account auth. Never the only way in. On failure, fall back to username/password or passkeys.
- The app never receives templates — only pass/fail, a Keychain item, or a hardware crypto result.
- Gate **secrets** with Keychain `SecAccessControl` or a Secure Enclave key, not a Boolean from `evaluatePolicy`.
- Facial account auth must use LocalAuthentication (App Review **2.5.13**), not ARKit. Alternate method required for users under 13.
- Required: `NSFaceIDUsageDescription`. Touch ID has no equivalent key. Do not put the app name in that string or in `localizedReason`.

## Which API

| Need | Use |
|------|-----|
| UI gate after the user is already known | `evaluatePolicy` or `LARight` (iOS 16+). `LocalAuthenticationView` is **macOS 13+ only**. |
| Prompt at a chosen moment, then use a key without a second sheet | `evaluateAccessControl` + `kSecUseAuthenticationContext` (WWDC22 10108) |
| Release a token/password | Keychain item with `SecAccessControl` |
| Sign/decrypt without exposing the private key | Secure Enclave key (`privateKeyUsage` + biometry flag) or `LAPersistedRight` |

Login sample policy: `.deviceOwnerAuthentication` (system passcode fallback). `.deviceOwnerAuthenticationWithBiometrics` returns `userFallback` for you to handle. Fresh `LAContext` per login.

## Current vs deprecated (iOS 18 / macOS 15)

- Prefer `context.domainState.biometry.stateHash` over deprecated `evaluatedPolicyDomainState`.
- Prefer companion policies (Watch/Mac/Vision) over deprecated `…WithWatch` / `…WithBiometricsOrWatch`.
- Prefer `biometryLockout` over deprecated `touchIDLockout` names.

After a state-hash change or `.biometryCurrentSet` invalidation, re-verify the **account**. Do not silently recreate the Keychain item.

## UX (HIG)

Name the method (“Sign In with Face ID”). Branch on `LABiometryType` after `canEvaluatePolicy`. Warn before Face ID (scan starts immediately). No in-app “enable Face ID” toggle. Do not call an app password a “passcode”. Prefer passkeys / Sign in with Apple for the account itself.
