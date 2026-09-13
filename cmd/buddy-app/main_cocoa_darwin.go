//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c -fno-objc-arc
#cgo LDFLAGS: -framework AppKit -framework Foundation
#import <AppKit/AppKit.h>
#include <signal.h>

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
*/
import "C"

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
)

func main() {
	runtime.LockOSThread()

	helper, err := resolveHelper()
	if err != nil {
		fatalf("buddy: %v", err)
	}

	cmd := exec.Command(helper, defaultArgs()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		fatalf("buddy: start %s: %v", helper, err)
	}
	C.buddySetHelperPgid(C.int(cmd.Process.Pid))

	go func() {
		_ = cmd.Wait()
		C.buddyAppStop()
	}()

	C.buddyAppRun()
}
