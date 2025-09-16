// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: Ryan Johnson

package vnc

import (
	"io"
)

// Encoding defines the interface for VNC framebuffer encoding methods.
type Encoding interface {
	Type() int32
	Read(*ClientConn, *Rectangle, io.Reader) (Encoding, error)
}

// PseudoEncoding defines the interface for VNC pseudo-encodings.
// Pseudo-encodings provide metadata or control information rather than pixel data.
type PseudoEncoding interface {
	Encoding

	IsPseudo() bool
	Handle(*ClientConn, *Rectangle) error
}
