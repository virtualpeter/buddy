//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fno-objc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation -framework CoreFoundation
#import <AppKit/AppKit.h>
#import <CoreFoundation/CoreFoundation.h>
#include <signal.h>
#include <stdlib.h>

static pid_t helperPgid;

void buddySetHelperPgid(int pid) {
	helperPgid = (pid_t)pid;
}

@interface BuddyAppDelegate : NSObject <NSApplicationDelegate>
@end

@implementation BuddyAppDelegate
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication *)sender {
	if (helperPgid > 0) {
		kill(-helperPgid, SIGTERM);
	}
	return NSTerminateNow;
}
- (BOOL)applicationShouldHandleReopen:(NSApplication *)sender hasVisibleWindows:(BOOL)flag {
	return NO;
}
@end

void buddyAppRun(void) {
	@autoreleasepool {
		NSApplication *app = [NSApplication sharedApplication];
		static BuddyAppDelegate *del;
		del = [[BuddyAppDelegate alloc] init];
		[app setDelegate:del];
		[app setActivationPolicy:NSApplicationActivationPolicyRegular];
		[app finishLaunching];
		[app run];
	}
}

void buddyAppStop(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp terminate:nil];
	});
}

// Works from any thread without an AppKit run loop (Finder launch path).
void buddyShowAlert(const char *title, const char *message) {
	CFStringRef t = CFStringCreateWithCString(NULL, title ? title : "Buddy", kCFStringEncodingUTF8);
	CFStringRef m = CFStringCreateWithCString(NULL, message ? message : "", kCFStringEncodingUTF8);
	CFUserNotificationDisplayAlert(
		0, kCFUserNotificationCautionAlertLevel,
		NULL, NULL, NULL,
		t, m,
		CFSTR("OK"), NULL, NULL, NULL);
	if (t) CFRelease(t);
	if (m) CFRelease(m);
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	runtime.LockOSThread()

	helper, err := resolveHelper()
	if err != nil {
		showFatal(fmt.Sprintf("buddy: %v", err))
	}

	// Run AppKit on the main thread; start Agent + helper off it so a fast
	// helper failure can still show a CFUserNotification without deadlocking.
	go func() {
		ensureNestedAgent()

		cmd := exec.Command(helper, defaultArgs()...)
		var errBuf bytes.Buffer
		cmd.Stdout = os.Stdout
		cmd.Stderr = io.MultiWriter(os.Stderr, &errBuf)
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			showAlert("Buddy", fmt.Sprintf("buddy: start %s: %v", helper, err))
			C.buddyAppStop()
			return
		}
		C.buddySetHelperPgid(C.int(cmd.Process.Pid))

		waitErr := cmd.Wait()
		if waitErr != nil {
			msg := strings.TrimSpace(errBuf.String())
			if msg == "" {
				msg = waitErr.Error()
			}
			showAlert("Buddy could not start", msg)
		}
		C.buddyAppStop()
	}()

	C.buddyAppRun()
}

func ensureNestedAgent() {
	if agentReachable() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	agentApp, err := nestedAgentPath(exe)
	if err != nil {
		return
	}
	if st, err := os.Stat(agentApp); err != nil || !st.IsDir() {
		return
	}

	// Launch Services puts the Agent in its own session (survives Buddy quit).
	_ = exec.Command("/usr/bin/open", "-n", agentApp).Start()
	if waitForAgent(8 * time.Second) {
		return
	}

	bin := filepath.Join(agentApp, "Contents", "MacOS", "BuddyAgent")
	if st, err := os.Stat(bin); err == nil && !st.IsDir() {
		cmd := exec.Command(bin)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		_ = cmd.Start()
		_ = waitForAgent(8 * time.Second)
	}
}

func showAlert(title, message string) {
	ct, cm := C.CString(title), C.CString(message)
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(cm))
	C.buddyShowAlert(ct, cm)
}

func showFatal(msg string) {
	showAlert("Buddy", msg)
	os.Exit(1)
}
