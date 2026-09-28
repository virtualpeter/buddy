//go:build darwin && secitem

package secrets

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Security -framework CoreFoundation -framework LocalAuthentication -framework Foundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
#import <LocalAuthentication/LocalAuthentication.h>
#import <Foundation/Foundation.h>
#include <stdlib.h>
#include <string.h>

static CFStringRef cfstr(const char *s) {
	if (s == NULL) {
		return NULL;
	}
	return CFStringCreateWithCString(kCFAllocatorDefault, s, kCFStringEncodingUTF8);
}

static char *cfstr_c(CFStringRef s) {
	if (s == NULL) {
		return NULL;
	}
	CFIndex len = CFStringGetMaximumSizeForEncoding(CFStringGetLength(s), kCFStringEncodingUTF8) + 1;
	char *buf = malloc((size_t)len);
	if (buf == NULL) {
		return NULL;
	}
	if (!CFStringGetCString(s, buf, len, kCFStringEncodingUTF8)) {
		free(buf);
		return NULL;
	}
	return buf;
}

char *buddy_secitem_errstr(int st) {
	CFStringRef s = SecCopyErrorMessageString((OSStatus)st, NULL);
	char *c = cfstr_c(s);
	if (s != NULL) {
		CFRelease(s);
	}
	return c;
}

// buddy_secitem_get_ex reads a generic password.
// presence=1 attaches an LAContext (userPresence ACL items prompt).
// reason is the localizedReason / operation prompt.
// reuse_secs is touchIDAuthenticationAllowableReuseDuration (0 = none).
int buddy_secitem_get_ex(const char *service, const char *account, int presence, const char *reason, double reuse_secs, char **out, size_t *out_len) {
	@autoreleasepool {
		CFStringRef svc = cfstr(service);
		CFStringRef acct = cfstr(account);
		LAContext *ctx = nil;
		CFMutableDictionaryRef query = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, svc);
		CFDictionarySetValue(query, kSecAttrAccount, acct);
		CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
		CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
		if (presence) {
			ctx = [[LAContext alloc] init];
			if (reuse_secs > 0) {
				ctx.touchIDAuthenticationAllowableReuseDuration = reuse_secs;
			}
			NSString *r = @"Unlock Buddy vault";
			if (reason != NULL && reason[0] != 0) {
				r = [NSString stringWithUTF8String:reason];
			}
			ctx.localizedReason = r;
			CFDictionarySetValue(query, kSecUseAuthenticationContext, (__bridge CFTypeRef)ctx);
		}
		CFTypeRef result = NULL;
		OSStatus st = SecItemCopyMatching(query, &result);
		CFRelease(query);
		CFRelease(svc);
		CFRelease(acct);
		if (st != errSecSuccess) {
			return (int)st;
		}
		CFDataRef data = (CFDataRef)result;
		CFIndex n = CFDataGetLength(data);
		*out = malloc((size_t)n + 1);
		if (*out == NULL) {
			CFRelease(result);
			return (int)errSecAllocate;
		}
		if (n > 0) {
			memcpy(*out, CFDataGetBytePtr(data), (size_t)n);
		}
		(*out)[n] = 0;
		*out_len = (size_t)n;
		CFRelease(result);
		return 0;
	}
}

// buddy_secitem_has returns 0 if the item exists (no data, skip auth UI when possible).
int buddy_secitem_has(const char *service, const char *account) {
	CFStringRef svc = cfstr(service);
	CFStringRef acct = cfstr(account);
	const void *keys[] = {kSecClass, kSecAttrService, kSecAttrAccount, kSecReturnAttributes, kSecMatchLimit, kSecUseAuthenticationUI};
	const void *vals[] = {kSecClassGenericPassword, svc, acct, kCFBooleanTrue, kSecMatchLimitOne, kSecUseAuthenticationUISkip};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 6, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus st = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(svc);
	CFRelease(acct);
	if (result != NULL) {
		CFRelease(result);
	}
	return (int)st;
}

// buddy_secitem_readable_without_auth returns 0 if the secret is readable with
// LAContext.interactionNotAllowed (no ACL / no prompt). errSecInteractionNotAllowed
// (-25308) means presence/ACL is required. Other negative OSStatus on error.
int buddy_secitem_readable_without_auth(const char *service, const char *account) {
	@autoreleasepool {
		CFStringRef svc = cfstr(service);
		CFStringRef acct = cfstr(account);
		LAContext *ctx = [[LAContext alloc] init];
		ctx.interactionNotAllowed = YES;
		CFMutableDictionaryRef query = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		CFDictionarySetValue(query, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(query, kSecAttrService, svc);
		CFDictionarySetValue(query, kSecAttrAccount, acct);
		CFDictionarySetValue(query, kSecReturnData, kCFBooleanTrue);
		CFDictionarySetValue(query, kSecMatchLimit, kSecMatchLimitOne);
		CFDictionarySetValue(query, kSecUseAuthenticationContext, (__bridge CFTypeRef)ctx);
		CFTypeRef result = NULL;
		OSStatus st = SecItemCopyMatching(query, &result);
		CFRelease(query);
		CFRelease(svc);
		CFRelease(acct);
		if (result != NULL) {
			CFRelease(result);
		}
		return (int)st;
	}
}

// buddy_secitem_set_ex stores a generic password.
// presence=1 uses SecAccessControl userPresence + AfterFirstUnlockThisDeviceOnly.
// presence=0 + after_first_unlock=1 uses AfterFirstUnlockThisDeviceOnly without ACL.
// AccessControl cannot be applied via SecItemUpdate — presence writes always replace.
int buddy_secitem_set_ex(const char *service, const char *account, const char *label, const char *value, size_t value_len, int presence, int after_first_unlock) {
	CFStringRef svc = cfstr(service);
	CFStringRef acct = cfstr(account);
	CFStringRef lbl = cfstr(label);
	CFDataRef data = CFDataCreate(NULL, (const UInt8 *)value, (CFIndex)value_len);

	const void *qkeys[] = {kSecClass, kSecAttrService, kSecAttrAccount};
	const void *qvals[] = {kSecClassGenericPassword, svc, acct};
	CFDictionaryRef query = CFDictionaryCreate(NULL, qkeys, qvals, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);

	OSStatus st;
	if (presence) {
		(void)SecItemDelete(query);
		st = errSecItemNotFound;
	} else {
		CFMutableDictionaryRef attrs = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		CFDictionarySetValue(attrs, kSecValueData, data);
		if (lbl != NULL) {
			CFDictionarySetValue(attrs, kSecAttrLabel, lbl);
		}
		st = SecItemUpdate(query, attrs);
		CFRelease(attrs);
	}
	if (st == errSecItemNotFound) {
		CFMutableDictionaryRef item = CFDictionaryCreateMutable(NULL, 0, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		CFDictionarySetValue(item, kSecClass, kSecClassGenericPassword);
		CFDictionarySetValue(item, kSecAttrService, svc);
		CFDictionarySetValue(item, kSecAttrAccount, acct);
		CFDictionarySetValue(item, kSecValueData, data);
		if (lbl != NULL) {
			CFDictionarySetValue(item, kSecAttrLabel, lbl);
		}
		if (presence) {
			CFErrorRef cerr = NULL;
			SecAccessControlRef sac = SecAccessControlCreateWithFlags(
				kCFAllocatorDefault,
				kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly,
				kSecAccessControlUserPresence,
				&cerr);
			if (sac == NULL) {
				if (cerr != NULL) {
					CFRelease(cerr);
				}
				CFRelease(item);
				CFRelease(query);
				CFRelease(data);
				CFRelease(svc);
				CFRelease(acct);
				if (lbl != NULL) {
					CFRelease(lbl);
				}
				return (int)errSecParam;
			}
			CFDictionarySetValue(item, kSecAttrAccessControl, sac);
			CFRelease(sac);
			CFDictionarySetValue(item, kSecAttrComment, CFSTR("buddy-userPresence-v1"));
		} else if (after_first_unlock) {
			CFDictionarySetValue(item, kSecAttrAccessible, kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly);
		}
		st = SecItemAdd(item, NULL);
		CFRelease(item);
	}
	CFRelease(query);
	CFRelease(data);
	CFRelease(svc);
	CFRelease(acct);
	if (lbl != NULL) {
		CFRelease(lbl);
	}
	return (int)st;
}

int buddy_secitem_delete(const char *service, const char *account) {
	CFStringRef svc = cfstr(service);
	CFStringRef acct = cfstr(account);
	const void *keys[] = {kSecClass, kSecAttrService, kSecAttrAccount};
	const void *vals[] = {kSecClassGenericPassword, svc, acct};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	OSStatus st = SecItemDelete(query);
	CFRelease(query);
	CFRelease(svc);
	CFRelease(acct);
	return (int)st;
}

int buddy_secitem_list(const char *service, char **out) {
	CFStringRef svc = cfstr(service);
	const void *keys[] = {kSecClass, kSecAttrService, kSecReturnAttributes, kSecMatchLimit, kSecUseAuthenticationUI};
	const void *vals[] = {kSecClassGenericPassword, svc, kCFBooleanTrue, kSecMatchLimitAll, kSecUseAuthenticationUISkip};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 5, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFTypeRef result = NULL;
	OSStatus st = SecItemCopyMatching(query, &result);
	CFRelease(query);
	CFRelease(svc);
	if (st == errSecItemNotFound) {
		*out = strdup("");
		return 0;
	}
	if (st != errSecSuccess) {
		return (int)st;
	}
	CFArrayRef arr = (CFArrayRef)result;
	CFIndex n = CFArrayGetCount(arr);
	CFMutableStringRef acc = CFStringCreateMutable(NULL, 0);
	for (CFIndex i = 0; i < n; i++) {
		CFDictionaryRef d = (CFDictionaryRef)CFArrayGetValueAtIndex(arr, i);
		CFStringRef a = (CFStringRef)CFDictionaryGetValue(d, kSecAttrAccount);
		if (a == NULL) {
			continue;
		}
		if (CFStringGetLength(acc) > 0) {
			CFStringAppendCString(acc, "\n", kCFStringEncodingUTF8);
		}
		CFStringAppend(acc, a);
	}
	*out = cfstr_c(acc);
	CFRelease(acc);
	CFRelease(result);
	if (*out == NULL) {
		return (int)errSecAllocate;
	}
	return 0;
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"
)

const (
	keychainService    = keychainServiceSigned
	errSecItemNotFound = -25300
	// errSecInteractionNotAllowed: auth UI required but disallowed (UIFail/Skip).
	errSecInteractionNotAllowed = -25308
	// Do not reuse Mac unlock for vault DEK — process session already avoids
	// repeat prompts; reuse made presence look like a no-op after login.
	keychainReuseSecs = 0.0
)

// KeychainAccess is "framework": this binary uses Security.framework.
func KeychainAccess() string { return "framework" }

// KeychainStore stores secrets via Security.framework (signed release binary).
type KeychainStore struct{}

// NewKeychainStore creates a Security.framework-backed secret store.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Description() string {
	return fmt.Sprintf("macOS Keychain (service %q, Security.framework)", keychainService)
}

func keychainMasterDescription() string {
	return fmt.Sprintf("macOS Keychain (service %q, account %q, Security.framework)", keychainService, masterKeychainAccount)
}

func keychainDEKDescription() string {
	mode := "userPresence ACL when entitled; else LocalAuthentication + AfterFirstUnlockThisDeviceOnly"
	if !VaultUnlockUsesPresence() {
		mode = "AfterFirstUnlockThisDeviceOnly"
	}
	return fmt.Sprintf("macOS Keychain (service %q, account %q, %s, Security.framework)", keychainService, dekKeychainAccount, mode)
}

func keychainTokenWrapDescription() string {
	return fmt.Sprintf("macOS Keychain (service %q, account %q, AfterFirstUnlockThisDeviceOnly, Security.framework)", keychainService, tokenWrapKeychainAccount)
}

func secitemGet(service, account, reason string, presence bool) (string, error) {
	cSvc := C.CString(service)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	var cReason *C.char
	if reason != "" {
		cReason = C.CString(reason)
		defer C.free(unsafe.Pointer(cReason))
	}
	pres := 0
	reuse := 0.0
	if presence {
		pres = 1
		reuse = keychainReuseSecs
	}
	var out *C.char
	var n C.size_t
	st := C.buddy_secitem_get_ex(cSvc, cAcct, C.int(pres), cReason, C.double(reuse), &out, &n)
	defer cFree(out)
	if int(st) == errSecItemNotFound {
		return "", fmt.Errorf("keychain item %q not found", account)
	}
	if err := secitemError(st); err != nil {
		return "", err
	}
	return C.GoStringN(out, C.int(n)), nil
}

func secitemSet(service, account, label, value string, presence, afterFirstUnlock bool) error {
	cSvc := C.CString(service)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	var cVal *C.char
	if len(value) > 0 {
		cVal = (*C.char)(C.CBytes([]byte(value)))
		defer C.free(unsafe.Pointer(cVal))
	}
	var cLbl *C.char
	if label != "" {
		cLbl = C.CString(label)
		defer C.free(unsafe.Pointer(cLbl))
	}
	pres, afu := 0, 0
	if presence {
		pres = 1
	}
	if afterFirstUnlock {
		afu = 1
	}
	st := C.buddy_secitem_set_ex(cSvc, cAcct, cLbl, cVal, C.size_t(len(value)), C.int(pres), C.int(afu))
	return secitemError(st)
}

func secitemDeleteAccount(account string) error {
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	st := C.buddy_secitem_delete(cSvc, cAcct)
	if int(st) == errSecItemNotFound {
		return nil
	}
	return secitemError(st)
}

func secitemHas(account string) (bool, error) {
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	st := C.buddy_secitem_has(cSvc, cAcct)
	if int(st) == errSecItemNotFound {
		return false, nil
	}
	if int(st) == 0 {
		return true, nil
	}
	// Interaction skipped / auth required still means the item may exist.
	if int(st) == errSecInteractionNotAllowed || int(st) == -128 /* userCanceled */ {
		return true, nil
	}
	if err := secitemError(st); err != nil {
		return false, err
	}
	return true, nil
}

// secitemUnprotectedReadable reports whether the item's secret can be read
// without authentication UI (legacy AfterFirstUnlock, no userPresence ACL).
func secitemUnprotectedReadable(account string) (unprotected bool, missing bool, err error) {
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	st := C.buddy_secitem_readable_without_auth(cSvc, cAcct)
	switch int(st) {
	case 0:
		return true, false, nil
	case errSecItemNotFound:
		return false, true, nil
	case errSecInteractionNotAllowed, -25293 /* errSecAuthFailed */ :
		return false, false, nil
	default:
		return false, false, secitemError(st)
	}
}

func cFree(p *C.char) {
	if p != nil {
		C.free(unsafe.Pointer(p))
	}
}

func secitemError(st C.int) error {
	if st == 0 {
		return nil
	}
	msg := C.buddy_secitem_errstr(st)
	defer cFree(msg)
	if msg != nil {
		return fmt.Errorf("keychain: %s (%d)", C.GoString(msg), int(st))
	}
	return fmt.Errorf("keychain: status %d", int(st))
}

func keychainMasterGet() (string, error) {
	v, err := secitemGet(keychainService, masterKeychainAccount, "", false)
	if err != nil {
		return "", fmt.Errorf("recovery passphrase not set — run 'buddy cred set passphrase': %w", err)
	}
	return v, nil
}

func keychainMasterGetPresence(reason string) (string, error) {
	_ = reason
	return keychainMasterGet()
}

func keychainMasterSet(value string) error {
	return secitemSet(keychainService, masterKeychainAccount, keychainLabelSigned, value, false, false)
}

func keychainMasterSetPresence(value, reason string) error {
	_ = reason
	return keychainMasterSet(value)
}

func keychainMasterDelete() error {
	return secitemDeleteAccount(masterKeychainAccount)
}

func keychainDEKGet() (string, error) {
	return keychainDEKGetPresence(DefaultUnlockReason)
}

func keychainDEKGetPresence(reason string) (string, error) {
	if !VaultUnlockUsesPresence() {
		v, err := secitemGet(keychainService, dekKeychainAccount, reason, false)
		if err != nil {
			return "", fmt.Errorf("vault key not set: %w", err)
		}
		return v, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = DefaultUnlockReason
	}

	// Probe without UI. macOS does not return kSecAttrAccessControl in attribute dumps.
	unprotected, missing, err := secitemUnprotectedReadable(dekKeychainAccount)
	if err != nil {
		return "", fmt.Errorf("vault key not set: %w", err)
	}
	if missing {
		return "", fmt.Errorf("vault key not set: keychain item %q not found", dekKeychainAccount)
	}

	if unprotected {
		legacy, lerr := secitemGet(keychainService, dekKeychainAccount, reason, false)
		if lerr != nil {
			return "", fmt.Errorf("vault key not set: %w", lerr)
		}
		// Best-effort ACL migrate (needs keychain-access-groups / .app packaging).
		if merr := migrateDEKToPresence(legacy, reason); merr == nil {
			if v, gerr := secitemGet(keychainService, dekKeychainAccount, reason, true); gerr == nil {
				return v, nil
			}
		}
		// Developer ID CLI: SecAccessControl userPresence is unavailable (-34018).
		// Gate DEK release with LocalAuthentication, then read unprotected item.
		if err := confirmDeviceOwner(reason); err != nil {
			return "", err
		}
		return legacy, nil
	}

	v, err := secitemGet(keychainService, dekKeychainAccount, reason, true)
	if err != nil {
		return "", fmt.Errorf("vault key not set: %w", err)
	}
	return v, nil
}

func migrateDEKToPresence(value, reason string) error {
	_ = reason
	_ = secitemDeleteAccount(dekKeychainAccount)
	err := secitemSet(keychainService, dekKeychainAccount, keychainLabelSigned, value, true, true)
	if err != nil {
		// Never leave the vault without a DEK if ACL write is rejected.
		if rerr := secitemSet(keychainService, dekKeychainAccount, keychainLabelSigned, value, false, true); rerr != nil {
			return fmt.Errorf("%w (restore unprotected dek: %v)", err, rerr)
		}
		return err
	}
	return nil
}

func keychainDEKSet(value string) error {
	return keychainDEKSetPresence(value, "Buddy wants to store the vault key")
}

func keychainDEKSetPresence(value, reason string) error {
	_ = reason
	if !VaultUnlockUsesPresence() {
		_ = secitemDeleteAccount(dekKeychainAccount)
		return secitemSet(keychainService, dekKeychainAccount, keychainLabelSigned, value, false, true)
	}
	_ = secitemDeleteAccount(dekKeychainAccount)
	err := secitemSet(keychainService, dekKeychainAccount, keychainLabelSigned, value, true, true)
	if err == nil {
		return nil
	}
	// Fall back to AfterFirstUnlock when the platform rejects AccessControl.
	if rerr := secitemSet(keychainService, dekKeychainAccount, keychainLabelSigned, value, false, true); rerr != nil {
		return fmt.Errorf("%w (fallback unprotected dek: %v)", err, rerr)
	}
	return nil // stored unprotected; unlock path uses confirmDeviceOwner
}

func keychainDEKDelete() error {
	return secitemDeleteAccount(dekKeychainAccount)
}

func keychainTokenWrapGet() (string, error) {
	return secitemGet(keychainService, tokenWrapKeychainAccount, "", false)
}

func keychainTokenWrapSet(value string) error {
	_ = secitemDeleteAccount(tokenWrapKeychainAccount)
	return secitemSet(keychainService, tokenWrapKeychainAccount, keychainLabelSigned, value, false, true)
}

func keychainTokenWrapDelete() error {
	return secitemDeleteAccount(tokenWrapKeychainAccount)
}

func (k *KeychainStore) Get(credName string, kind Kind) (string, error) {
	account := keychainAccount(credName, kind)
	val, err := secitemGet(keychainService, account, DefaultUnlockReason, VaultUnlockUsesPresence())
	if err != nil {
		return "", fmt.Errorf("secret %q not found in keychain for credential %q", kind, credName)
	}
	return val, nil
}

func (k *KeychainStore) Set(credName string, kind Kind, value string) error {
	account := keychainAccount(credName, kind)
	_ = secitemDeleteAccount(account)
	return secitemSet(keychainService, account, keychainLabelSigned, value, VaultUnlockUsesPresence(), false)
}

func (k *KeychainStore) Delete(credName string, kind Kind) error {
	return secitemDeleteAccount(keychainAccount(credName, kind))
}

func (k *KeychainStore) Has(credName string, kind Kind) (bool, error) {
	return secitemHas(keychainAccount(credName, kind))
}

func (k *KeychainStore) List() ([]Entry, error) {
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	var out *C.char
	st := C.buddy_secitem_list(cSvc, &out)
	defer cFree(out)
	if err := secitemError(st); err != nil {
		return nil, err
	}
	raw := C.GoString(out)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var entries []Entry
	seen := map[string]struct{}{}
	for _, line := range strings.Split(raw, "\n") {
		acct := strings.TrimSpace(line)
		if acct == "" {
			continue
		}
		parts := strings.SplitN(acct, "/", 2)
		if len(parts) != 2 {
			continue
		}
		kind, err := ParseKind(parts[1])
		if err != nil {
			continue
		}
		key := parts[0] + "/" + kind.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, Entry{AuthName: parts[0], Kind: kind})
	}
	return entries, nil
}
