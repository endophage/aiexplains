package cmd

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>

void setupAppMenu(const char *appName) {
	NSString *name = [NSString stringWithUTF8String:appName];

	NSMenu *menubar = [[NSMenu alloc] initWithTitle:@""];
	[NSApp setMainMenu:menubar];

	// App menu (first item's title is ignored by macOS; the app name is used)
	NSMenuItem *appItem = [[NSMenuItem alloc] initWithTitle:name action:nil keyEquivalent:@""];
	[menubar addItem:appItem];

	NSMenu *appMenu = [[NSMenu alloc] initWithTitle:name];
	[appItem setSubmenu:appMenu];

	NSString *quitTitle = [@"Quit " stringByAppendingString:name];
	NSMenuItem *quitItem = [[NSMenuItem alloc]
		initWithTitle:quitTitle
		action:@selector(terminate:)
		keyEquivalent:@"q"];
	[appMenu addItem:quitItem];
}
*/
import "C"

func setupMenu() {
	C.setupAppMenu(C.CString("AIExplains"))
}
