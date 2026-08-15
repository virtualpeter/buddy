//go:build darwin && secitem

package secrets

/*
#cgo LDFLAGS: -framework Security -framework CoreFoundation
#include <CoreFoundation/CoreFoundation.h>
#include <Security/Security.h>
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

int buddy_secitem_get(const char *service, const char *account, char **out, size_t *out_len) {
	CFStringRef svc = cfstr(service);
	CFStringRef acct = cfstr(account);
	const void *keys[] = {kSecClass, kSecAttrService, kSecAttrAccount, kSecReturnData, kSecMatchLimit};
	const void *vals[] = {kSecClassGenericPassword, svc, acct, kCFBooleanTrue, kSecMatchLimitOne};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 5, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
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

int buddy_secitem_set(const char *service, const char *account, const char *label, const char *value, size_t value_len) {
	CFStringRef svc = cfstr(service);
	CFStringRef acct = cfstr(account);
	CFStringRef lbl = cfstr(label);
	CFDataRef data = CFDataCreate(NULL, (const UInt8 *)value, (CFIndex)value_len);

	const void *qkeys[] = {kSecClass, kSecAttrService, kSecAttrAccount};
	const void *qvals[] = {kSecClassGenericPassword, svc, acct};
	CFDictionaryRef query = CFDictionaryCreate(NULL, qkeys, qvals, 3, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	CFDictionaryRef attrs;
	if (lbl != NULL) {
		const void *akeys[] = {kSecValueData, kSecAttrLabel};
		const void *avals[] = {data, lbl};
		attrs = CFDictionaryCreate(NULL, akeys, avals, 2, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	} else {
		const void *akey = kSecValueData;
		const void *aval = data;
		attrs = CFDictionaryCreate(NULL, &akey, &aval, 1, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
	}
	OSStatus st = SecItemUpdate(query, attrs);
	CFRelease(attrs);
	if (st == errSecItemNotFound) {
		CFDictionaryRef item;
		if (lbl != NULL) {
			const void *ikeys[] = {kSecClass, kSecAttrService, kSecAttrAccount, kSecAttrLabel, kSecValueData};
			const void *ivals[] = {kSecClassGenericPassword, svc, acct, lbl, data};
			item = CFDictionaryCreate(NULL, ikeys, ivals, 5, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
		} else {
			const void *ikeys[] = {kSecClass, kSecAttrService, kSecAttrAccount, kSecValueData};
			const void *ivals[] = {kSecClassGenericPassword, svc, acct, data};
			item = CFDictionaryCreate(NULL, ikeys, ivals, 4, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
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
	const void *keys[] = {kSecClass, kSecAttrService, kSecReturnAttributes, kSecMatchLimit};
	const void *vals[] = {kSecClassGenericPassword, svc, kCFBooleanTrue, kSecMatchLimitAll};
	CFDictionaryRef query = CFDictionaryCreate(NULL, keys, vals, 4, &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
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
)

// KeychainStore stores secrets via Security.framework (signed release binary).
type KeychainStore struct{}

// NewKeychainStore creates a Security.framework-backed secret store.
func NewKeychainStore() *KeychainStore {
	return &KeychainStore{}
}

func (k *KeychainStore) Description() string {
	return fmt.Sprintf("macOS Keychain (service %q, Security.framework)", keychainService)
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

func (k *KeychainStore) Get(credName string, kind Kind) (string, error) {
	account := keychainAccount(credName, kind)
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	var out *C.char
	var n C.size_t
	st := C.buddy_secitem_get(cSvc, cAcct, &out, &n)
	defer cFree(out)
	if int(st) == errSecItemNotFound {
		return "", fmt.Errorf("secret %q not found in keychain for credential %q", kind, credName)
	}
	if err := secitemError(st); err != nil {
		return "", err
	}
	return C.GoStringN(out, C.int(n)), nil
}

func (k *KeychainStore) Set(credName string, kind Kind, value string) error {
	account := keychainAccount(credName, kind)
	cSvc := C.CString(keychainService)
	defer C.free(unsafe.Pointer(cSvc))
	cAcct := C.CString(account)
	defer C.free(unsafe.Pointer(cAcct))
	var cVal *C.char
	if len(value) > 0 {
		cVal = (*C.char)(C.CBytes([]byte(value)))
		defer C.free(unsafe.Pointer(cVal))
	}
	cLbl := C.CString(keychainLabelSigned)
	defer C.free(unsafe.Pointer(cLbl))
	st := C.buddy_secitem_set(cSvc, cAcct, cLbl, cVal, C.size_t(len(value)))
	return secitemError(st)
}

func (k *KeychainStore) Delete(credName string, kind Kind) error {
	account := keychainAccount(credName, kind)
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

func (k *KeychainStore) Has(credName string, kind Kind) (bool, error) {
	_, err := k.Get(credName, kind)
	if err != nil {
		return false, nil
	}
	return true, nil
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
