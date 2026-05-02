//go:build !android

package platsvc

import "errors"

// ErrShareUnsupported is returned by ShareText/ShareFile on platforms that
// don't have a native share-sheet (everything except Android).
var ErrShareUnsupported = errors.New("native share is only available on Android")

// ShareText opens the OS share sheet with plain-text content. On non-Android
// platforms this is a no-op and returns ErrShareUnsupported.
func ShareText(_, _ string) error { return ErrShareUnsupported }

// ShareFile opens the OS share sheet for a single file. On non-Android
// platforms this is a no-op and returns ErrShareUnsupported.
func ShareFile(_, _, _ string) error { return ErrShareUnsupported }

// ShareFiles opens the OS share sheet with multiple file attachments. On
// non-Android platforms this is a no-op and returns ErrShareUnsupported.
func ShareFiles(_ []string, _, _ string) error { return ErrShareUnsupported }

// ShareSupported reports whether ShareText/ShareFile are functional.
func ShareSupported() bool { return false }
