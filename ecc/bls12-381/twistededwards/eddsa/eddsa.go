// Copyright 2020 ConsenSys Software Inc.
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

package eddsa

import (
	"crypto/subtle"
	"errors"
	"hash"
	"io"
	"math/big"

	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/twistededwards"
	"github.com/consensys/gnark-crypto/signature"
	"golang.org/x/crypto/blake2b"
)

var errNotOnCurve = errors.New("point not on curve")

const (
	sizeFr         = fr.Bytes
	sizePublicKey  = sizeFr
	sizeSignature  = 2 * sizeFr
	sizePrivateKey = 2*sizeFr + 32
)

// PublicKey eddsa signature object
// cf https://en.wikipedia.org/wiki/EdDSA for notation
type PublicKey struct {
	A twistededwards.PointAffine
}

// PrivateKey private key of an eddsa instance
type PrivateKey struct {
	PublicKey PublicKey    // copy of the associated public key
	scalar    [sizeFr]byte // secret scalar, in big Endian
	randSrc   [32]byte     // source
}

// Signature represents an eddsa signature
// cf https://en.wikipedia.org/wiki/EdDSA for notation
type Signature struct {
	R twistededwards.PointAffine
	S [sizeFr]byte
}

func init() {
	signature.Register(signature.EDDSA_BLS12_381, GenerateKeyInterfaces)
}

// GenerateKey generates a public and private key pair.
func GenerateKey(r io.Reader) (PrivateKey, error) {

	c := twistededwards.GetEdwardsCurve()

	var pub PublicKey
	var priv PrivateKey

	// hash(h) = private_key || random_source, on 32 bytes each
	seed := make([]byte, 32)
	_, err := r.Read(seed)
	if err != nil {
		return priv, err
	}
	h := blake2b.Sum512(seed[:])
	for i := 0; i < 32; i++ {
		priv.randSrc[i] = h[i+32]
	}

	// prune the key
	// https://tools.ietf.org/html/rfc8032#section-5.1.5, key generation

	h[0] &= 0xF8
	h[31] &= 0x7F
	h[31] |= 0x40

	// reverse first bytes because setBytes interpret stream as big endian
	// but in eddsa specs s is the first 32 bytes in little endian
	for i, j := 0, sizeFr; i < j; i, j = i+1, j-1 {

		h[i], h[j] = h[j], h[i]

	}

	copy(priv.scalar[:], h[:sizeFr])

	var bscalar big.Int
	bscalar.SetBytes(priv.scalar[:])
	pub.A.ScalarMul(&c.Base, &bscalar)

	priv.PublicKey = pub

	return priv, nil
}

// GenerateKeyInterfaces generate interfaces for the public/private key.
// This purpose of this function is to be registered in the list of signature schemes.
func GenerateKeyInterfaces(r io.Reader) (signature.Signer, error) {
	priv, err := GenerateKey(r)
	return &priv, err
}

// Equal compares 2 public keys
func (pub *PublicKey) Equal(other signature.PublicKey) bool {
	bpk := pub.Bytes()
	bother := other.Bytes()
	return subtle.ConstantTimeCompare(bpk, bother) == 1
}

// Public returns the public key associated to the private key.
// From Signer interface defined in gnark/crypto/signature.
func (privKey *PrivateKey) Public() signature.PublicKey {
	var pub PublicKey
	pub.A.Set(&privKey.PublicKey.A)
	return &pub
}

// Sign sign a message
// Pure Eddsa version (see https://tools.ietf.org/html/rfc8032#page-8)
func (privKey *PrivateKey) Sign(message []byte, hFunc hash.Hash) ([]byte, error) {

	curveParams := twistededwards.GetEdwardsCurve()

	var res Signature

	// blinding factor for the private key
	// blindingFactorBigInt must be the same size as the private key,
	// blindingFactorBigInt = h(randomness_source||message)[:sizeFr]
	var blindingFactorBigInt big.Int

	// randSrc = privKey.randSrc || msg (-> message = MSB message .. LSB message)
	randSrc := make([]byte, 32+len(message))
	for i, v := range privKey.randSrc {
		randSrc[i] = v
	}
	copy(randSrc[32:], message)

	// randBytes = H(randSrc)
	blindingFactorBytes := blake2b.Sum512(randSrc[:]) // TODO ensures that the hash used to build the key and the one used here is the same
	blindingFactorBigInt.SetBytes(blindingFactorBytes[:sizeFr])

	// compute R = randScalar*Base
	res.R.ScalarMul(&curveParams.Base, &blindingFactorBigInt)
	if !res.R.IsOnCurve() {
		return nil, errNotOnCurve
	}

	// compute H(R, A, M), all parameters in data are in Montgomery form
	resRX := res.R.X.Bytes()
	resRY := res.R.Y.Bytes()
	resAX := privKey.PublicKey.A.X.Bytes()
	resAY := privKey.PublicKey.A.Y.Bytes()
	sizeDataToHash := 4*sizeFr + len(message)
	dataToHash := make([]byte, sizeDataToHash)
	copy(dataToHash[:], resRX[:])
	copy(dataToHash[sizeFr:], resRY[:])
	copy(dataToHash[2*sizeFr:], resAX[:])
	copy(dataToHash[3*sizeFr:], resAY[:])
	copy(dataToHash[4*sizeFr:], message)
	hFunc.Reset()
	_, err := hFunc.Write(dataToHash[:])
	if err != nil {
		return nil, err
	}

	var hramInt big.Int
	hramBin := hFunc.Sum(nil)
	hramInt.SetBytes(hramBin)

	// Compute s = randScalarInt + H(R,A,M)*S
	// going with big int to do ops mod curve order
	var bscalar, bs big.Int
	bscalar.SetBytes(privKey.scalar[:])
	bs.Mul(&hramInt, &bscalar).
		Add(&bs, &blindingFactorBigInt).
		Mod(&bs, &curveParams.Order)
	sb := bs.Bytes()
	if len(sb) < sizeFr {
		offset := make([]byte, sizeFr-len(sb))
		sb = append(offset, sb...)
	}
	copy(res.S[:], sb[:])

	return res.Bytes(), nil
}

// Verify verifies an eddsa signature
func (pub *PublicKey) Verify(sigBin, message []byte, hFunc hash.Hash) (bool, error) {

	curveParams := twistededwards.GetEdwardsCurve()

	// verify that pubKey and R are on the curve
	if !pub.A.IsOnCurve() {
		return false, errNotOnCurve
	}

	// Deserialize the signature
	var sig Signature
	if _, err := sig.SetBytes(sigBin); err != nil {
		return false, err
	}

	// compute H(R, A, M), all parameters in data are in Montgomery form
	sigRX := sig.R.X.Bytes()
	sigRY := sig.R.Y.Bytes()
	sigAX := pub.A.X.Bytes()
	sigAY := pub.A.Y.Bytes()
	sizeDataToHash := 4*sizeFr + len(message)
	dataToHash := make([]byte, sizeDataToHash)
	copy(dataToHash[:], sigRX[:])
	copy(dataToHash[sizeFr:], sigRY[:])
	copy(dataToHash[2*sizeFr:], sigAX[:])
	copy(dataToHash[3*sizeFr:], sigAY[:])
	copy(dataToHash[4*sizeFr:], message)
	hFunc.Reset()
	if _, err := hFunc.Write(dataToHash[:]); err != nil {
		return false, err
	}

	var hramInt big.Int
	hramBin := hFunc.Sum(nil)
	hramInt.SetBytes(hramBin)

	// lhs = cofactor*S*Base
	var lhs twistededwards.PointAffine
	var bCofactor, bs big.Int
	curveParams.Cofactor.ToBigInt(&bCofactor)
	bs.SetBytes(sig.S[:])
	lhs.ScalarMul(&curveParams.Base, &bs).
		ScalarMul(&lhs, &bCofactor)

	if !lhs.IsOnCurve() {
		return false, errNotOnCurve
	}

	// rhs = cofactor*(R + H(R,A,M)*A)
	var rhs twistededwards.PointAffine
	rhs.ScalarMul(&pub.A, &hramInt).
		Add(&rhs, &sig.R).
		ScalarMul(&rhs, &bCofactor)
	if !rhs.IsOnCurve() {
		return false, errNotOnCurve
	}

	// verifies that cofactor*S*Base=cofactor*(R + H(R,A,M)*A)
	if !lhs.X.Equal(&rhs.X) || !lhs.Y.Equal(&rhs.Y) {
		return false, nil
	}

	return true, nil
}