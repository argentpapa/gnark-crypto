// Copyright 2020 Consensys Software Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by consensys/gnark-crypto DO NOT EDIT

package ecdsa

import (
	"crypto/subtle"
	"errors"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
	"io"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bn254"
)

var errWrongSize = errors.New("wrong size buffer")
var errRBiggerThanRMod = errors.New("r >= r_mod")
var errSBiggerThanRMod = errors.New("s >= r_mod")
var errZero = errors.New("zero value")

// Bytes returns the binary representation of the public key
// follows https://tools.ietf.org/html/rfc8032#section-3.1
// and returns a compressed representation of the point (x,y)
//
// x, y are the coordinates of the point
// on the curve as big endian integers.
// compressed representation store x with a parity bit to recompute y
func (pk *PublicKey) Bytes() []byte {
	var res [sizePublicKey]byte
	pkBin := pk.A.Bytes()
	subtle.ConstantTimeCopy(1, res[:sizePublicKey], pkBin[:])
	return res[:]
}

// SetBytes sets p from binary representation in buf.
// buf represents a public key as x||y where x, y are
// interpreted as big endian binary numbers corresponding
// to the coordinates of a point on the curve.
// It returns the number of bytes read from the buffer.
func (pk *PublicKey) SetBytes(buf []byte) (int, error) {
	n := 0
	if len(buf) < sizePublicKey {
		return n, io.ErrShortBuffer
	}
	if _, err := pk.A.SetBytes(buf[:sizePublicKey]); err != nil {
		return 0, err
	}
	n += sizeFp
	return n, nil
}

// RecoverFrom recovers the public key from the message msg, recovery
// information v and decompose signature {r,s}. If recovery succeeded, the
// methods sets the current public key to the recovered value. Otherwise returns
// error and leaves current public key unchanged.
func (pk *PublicKey) RecoverFrom(msg []byte, v uint, r, s *big.Int) error {
	if s.Cmp(fr.Modulus()) >= 0 {
		return errors.New("s is larger than modulus")
	}
	if s.Cmp(big.NewInt(0)) <= 0 {
		return errors.New("s is negative")
	}
	P, err := recoverP(v, r)
	if err != nil {
		return err
	}
	z := HashToInt(msg)
	rinv := new(big.Int).ModInverse(r, fr.Modulus())
	u1 := new(big.Int).Mul(z, rinv)
	u1.Neg(u1)
	u1.Mod(u1, fr.Modulus())
	u2 := new(big.Int).Mul(s, rinv)
	u2.Mod(u2, fr.Modulus())
	var Q bn254.G1Jac
	Q.JointScalarMultiplicationBase(P, u1, u2)
	pk.A.FromJacobian(&Q)
	return nil
}

// Bytes returns the binary representation of pk,
// as byte array publicKey||scalar
// where publicKey is as publicKey.Bytes(), and
// scalar is in big endian, of size sizeFr.
func (privKey *PrivateKey) Bytes() []byte {
	var res [sizePrivateKey]byte
	pubkBin := privKey.PublicKey.A.Bytes()
	subtle.ConstantTimeCopy(1, res[:sizePublicKey], pubkBin[:])
	subtle.ConstantTimeCopy(1, res[sizePublicKey:sizePrivateKey], privKey.scalar[:])
	return res[:]
}

// SetBytes sets pk from buf, where buf is interpreted
// as  publicKey||scalar
// where publicKey is as publicKey.Bytes(), and
// scalar is in big endian, of size sizeFr.
// It returns the number byte read.
func (privKey *PrivateKey) SetBytes(buf []byte) (int, error) {
	n := 0
	if len(buf) < sizePrivateKey {
		return n, io.ErrShortBuffer
	}
	if _, err := privKey.PublicKey.A.SetBytes(buf[:sizePublicKey]); err != nil {
		return 0, err
	}
	n += sizePublicKey
	subtle.ConstantTimeCopy(1, privKey.scalar[:], buf[sizePublicKey:sizePrivateKey])
	n += sizeFr
	return n, nil
}

// Bytes returns the binary representation of sig
// as a byte array of size 2*sizeFr r||s
func (sig *Signature) Bytes() []byte {
	var res [sizeSignature]byte
	subtle.ConstantTimeCopy(1, res[:sizeFr], sig.R[:])
	subtle.ConstantTimeCopy(1, res[sizeFr:], sig.S[:])
	return res[:]
}

// SetBytes sets sig from a buffer in binary.
// buf is read interpreted as r||s
// It returns the number of bytes read from buf.
func (sig *Signature) SetBytes(buf []byte) (int, error) {
	n := 0
	if len(buf) != sizeSignature {
		return n, errWrongSize
	}

	// S, R < R_mod (to avoid malleability)
	frMod := fr.Modulus()
	zero := big.NewInt(0)
	bufBigInt := new(big.Int)
	bufBigInt.SetBytes(buf[:sizeFr])
	if bufBigInt.Cmp(zero) == 0 {
		return 0, errZero
	}
	if bufBigInt.Cmp(frMod) != -1 {
		return 0, errRBiggerThanRMod
	}
	bufBigInt.SetBytes(buf[sizeFr : 2*sizeFr])
	if bufBigInt.Cmp(zero) == 0 {
		return 0, errZero
	}
	if bufBigInt.Cmp(frMod) != -1 {
		return 0, errSBiggerThanRMod
	}

	subtle.ConstantTimeCopy(1, sig.R[:], buf[:sizeFr])
	n += sizeFr
	subtle.ConstantTimeCopy(1, sig.S[:], buf[sizeFr:2*sizeFr])
	n += sizeFr
	return n, nil
}
