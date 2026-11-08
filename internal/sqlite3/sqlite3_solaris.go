// HomeWaf modification (2026-11-09): independent distribution; see NOTICE and docs/modernization.

// Copyright (C) 2019 Yasuhiro Matsumoto <mattn.jp@gmail.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//go:build solaris

package sqlite3

/*
#cgo CFLAGS: -D__EXTENSIONS__=1
#cgo LDFLAGS: -lc
*/
import "C"
