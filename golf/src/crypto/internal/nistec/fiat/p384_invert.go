// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code generated by addchain. DO NOT EDIT.

package fiat

// Invert sets e = 1/x, and returns e.
//
// If x == 0, Invert returns e = 0.
func (e *P384Element) Invert(x *P384Element) *P384Element {
	// Inversion is implemented as exponentiation with exponent p − 2.
	// The sequence of 15 multiplications and 383 squarings is derived from the
	// following addition chain generated with github.com/mmcloughlin/addchain v0.4.0.
	//
	//	_10     = 2*1
	//	_11     = 1 + _10
	//	_110    = 2*_11
	//	_111    = 1 + _110
	//	_111000 = _111 << 3
	//	_111111 = _111 + _111000
	//	x12     = _111111 << 6 + _111111
	//	x24     = x12 << 12 + x12
	//	x30     = x24 << 6 + _111111
	//	x31     = 2*x30 + 1
	//	x32     = 2*x31 + 1
	//	x63     = x32 << 31 + x31
	//	x126    = x63 << 63 + x63
	//	x252    = x126 << 126 + x126
	//	x255    = x252 << 3 + _111
	//	i397    = ((x255 << 33 + x32) << 94 + x30) << 2
	//	return    1 + i397
	//

	var z = new(P384Element).Set(e)
	var t0 = new(P384Element)
	var t1 = new(P384Element)
	var t2 = new(P384Element)
	var t3 = new(P384Element)

	z.Square(x)
	z.Mul(x, z)
	z.Square(z)
	t1.Mul(x, z)
	z.Square(t1)
	for s := 1; s < 3; s++ {
		z.Square(z)
	}
	z.Mul(t1, z)
	t0.Square(z)
	for s := 1; s < 6; s++ {
		t0.Square(t0)
	}
	t0.Mul(z, t0)
	t2.Square(t0)
	for s := 1; s < 12; s++ {
		t2.Square(t2)
	}
	t0.Mul(t0, t2)
	for s := 0; s < 6; s++ {
		t0.Square(t0)
	}
	z.Mul(z, t0)
	t0.Square(z)
	t2.Mul(x, t0)
	t0.Square(t2)
	t0.Mul(x, t0)
	t3.Square(t0)
	for s := 1; s < 31; s++ {
		t3.Square(t3)
	}
	t2.Mul(t2, t3)
	t3.Square(t2)
	for s := 1; s < 63; s++ {
		t3.Square(t3)
	}
	t2.Mul(t2, t3)
	t3.Square(t2)
	for s := 1; s < 126; s++ {
		t3.Square(t3)
	}
	t2.Mul(t2, t3)
	for s := 0; s < 3; s++ {
		t2.Square(t2)
	}
	t1.Mul(t1, t2)
	for s := 0; s < 33; s++ {
		t1.Square(t1)
	}
	t0.Mul(t0, t1)
	for s := 0; s < 94; s++ {
		t0.Square(t0)
	}
	z.Mul(z, t0)
	for s := 0; s < 2; s++ {
		z.Square(z)
	}
	z.Mul(x, z)

	return e.Set(z)
}
