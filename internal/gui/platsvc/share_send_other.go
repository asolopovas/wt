//go:build !android

package platsvc

import "errors"

var ErrShareUnsupported = errors.New("native share is only available on Android")

func ShareText(_, _ string) error { return ErrShareUnsupported }

func ShareFile(_, _, _ string) error { return ErrShareUnsupported }

func ShareFiles(_ []string, _, _ string) error { return ErrShareUnsupported }

func ShareSupported() bool { return false }
