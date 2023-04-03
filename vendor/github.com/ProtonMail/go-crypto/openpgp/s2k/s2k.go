// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package s2k implements the various OpenPGP string-to-key transforms as
// specified in RFC 4800 section 3.7.1, and Argon2 specified in
// draft-ietf-openpgp-crypto-refresh-08 section 3.7.1.4.
package s2k // import "github.com/ProtonMail/go-crypto/openpgp/s2k"

import (
	"crypto"
	"hash"
	"io"
	"strconv"

	"github.com/ProtonMail/go-crypto/openpgp/errors"
	"github.com/ProtonMail/go-crypto/openpgp/internal/algorithm"
	"golang.org/x/crypto/argon2"
)

type Mode uint8

// Defines the default S2KMode constants
//
//	0 (simple), 1(salted), 3(iterated), 4(argon2)
const (
	SimpleS2K         Mode = 0
	SaltedS2K              = 1
	IteratedSaltedS2K      = 3
	Argon2S2K              = 4
	GnuS2K                 = 101
)

const Argon2SaltSize int = 16

// Params contains all the parameters of the s2k packet
type Params struct {
	// mode is the mode of s2k function.
	// It can be 0 (simple), 1(salted), 3(iterated)
	// 2(reserved) 100-110(private/experimental).
	mode Mode
	// hashId is the ID of the hash function used in any of the modes
	hashId byte
	// salt is a byte array to use as a salt in hashing process or argon2
	salt []byte
	// countByte is used to determine how many rounds of hashing are to
	// be performed in s2k mode 3. See RFC 4880 Section 3.7.1.3.
	countByte byte
	// passes is a parameter in Argon2 to determine the number of iterations
	// See RFC the crypto refresh Section 3.7.1.4.
	passes byte
	// parallelism is a parameter in Argon2 to determine the degree of paralellism
	// See RFC the crypto refresh Section 3.7.1.4.
	parallelism byte
	// memoryExp is a parameter in Argon2 to determine the memory usage
	// i.e., 2 ** memoryExp kibibytes
	// See RFC the crypto refresh Section 3.7.1.4.
	memoryExp byte
}

// encodeCount converts an iterative "count" in the range 1024 to
// 65011712, inclusive, to an encoded count. The return value is the
// octet that is actually stored in the GPG file. encodeCount panics
// if i is not in the above range (encodedCount above takes care to
// pass i in the correct range). See RFC 4880 Section 3.7.7.1.
func encodeCount(i int) uint8 {
	if i < 65536 || i > 65011712 {
		panic("count arg i outside the required range")
	}

	for encoded := 96; encoded < 256; encoded++ {
		count := decodeCount(uint8(encoded))
		if count >= i {
			return uint8(encoded)
		}
	}

	return 255
}

// decodeCount returns the s2k mode 3 iterative "count" corresponding to
// the encoded octet c.
func decodeCount(c uint8) int {
	return (16 + int(c&15)) << (uint32(c>>4) + 6)
}

// encodeMemory converts the Argon2 "memory" in the range parallelism*8 to
// 2**31, inclusive, to an encoded memory. The return value is the
// octet that is actually stored in the GPG file. encodeMemory panics
// if is not in the above range 
// See OpenPGP crypto refresh Section 3.7.1.4.
func encodeMemory(memory uint32, parallelism uint8) uint8 {
	if memory < (8 * uint32(parallelism)) || memory > uint32(2147483648) {
		panic("Memory argument memory is outside the required range")
	}

	for exp := 3; exp < 31; exp++ {
		compare := decodeMemory(uint8(exp))
		if compare >= memory {
			return uint8(exp)
		}
	}

	return 31
}

// decodeMemory computes the decoded memory in kibibytes as 2**memoryExponent
func decodeMemory(memoryExponent uint8) uint32 {
	return uint32(1) << memoryExponent
}

// Simple writes to out the result of computing the Simple S2K function (RFC
// 4880, section 3.7.1.1) using the given hash and input passphrase.
func Simple(out []byte, h hash.Hash, in []byte) {
	Salted(out, h, in, nil)
}

var zero [1]byte

// Salted writes to out the result of computing the Salted S2K function (RFC
// 4880, section 3.7.1.2) using the given hash, input passphrase and salt.
func Salted(out []byte, h hash.Hash, in []byte, salt []byte) {
	done := 0
	var digest []byte

	for i := 0; done < len(out); i++ {
		h.Reset()
		for j := 0; j < i; j++ {
			h.Write(zero[:])
		}
		h.Write(salt)
		h.Write(in)
		digest = h.Sum(digest[:0])
		n := copy(out[done:], digest)
		done += n
	}
}

// Iterated writes to out the result of computing the Iterated and Salted S2K
// function (RFC 4880, section 3.7.1.3) using the given hash, input passphrase,
// salt and iteration count.
func Iterated(out []byte, h hash.Hash, in []byte, salt []byte, count int) {
	combined := make([]byte, len(in)+len(salt))
	copy(combined, salt)
	copy(combined[len(salt):], in)

	if count < len(combined) {
		count = len(combined)
	}

	done := 0
	var digest []byte
	for i := 0; done < len(out); i++ {
		h.Reset()
		for j := 0; j < i; j++ {
			h.Write(zero[:])
		}
		written := 0
		for written < count {
			if written+len(combined) > count {
				todo := count - written
				h.Write(combined[:todo])
				written = count
			} else {
				h.Write(combined)
				written += len(combined)
			}
		}
		digest = h.Sum(digest[:0])
		n := copy(out[done:], digest)
		done += n
	}
}

// Argon2 writes to out the key derived from the password (in) with the Argon2
// function (the crypto refresh, section 3.7.1.4)
func Argon2(out []byte, in []byte, salt []byte, passes uint8, paralellism uint8, memoryExp uint8) {
	key := argon2.IDKey(in, salt, uint32(passes), decodeMemory(memoryExp), paralellism, uint32(len(out)))
	copy(out[:], key)
}

// Generate generates valid parameters from given configuration.
// It will enforce the Iterated and Salted or Argon2 S2K method.
func Generate(rand io.Reader, c *Config) (*Params, error) {
	var params *Params
	if c != nil && c.S2KMode == Argon2S2K {
		// handle Argon2 case
		argonConfig := c.Argon2()
		params = &Params{
			mode:        Argon2S2K,
			salt:        make([]byte, Argon2SaltSize),
			passes:      argonConfig.Passes(),
			parallelism: argonConfig.Parallelism(),
			memoryExp:   argonConfig.EncodedMemory(),
		}
	} else {
		// handle IteratedSaltedS2K case
		hashId, ok := algorithm.HashToHashId(c.hash())
		if !ok {
			return nil, errors.UnsupportedError("no such hash")
		}

		params = &Params{
			mode:      IteratedSaltedS2K, // Enforce iterared + salted method if not Argon 2
			hashId:    hashId,
			salt:      make([]byte, 8),
			countByte: c.EncodedCount(),
		}
	}
	if _, err := io.ReadFull(rand, params.salt); err != nil {
		return nil, err
	}
	return params, nil
}

// Parse reads a binary specification for a string-to-key transformation from r
// and returns a function which performs that transform. If the S2K is a special
// GNU extension that indicates that the private key is missing, then the error
// returned is errors.ErrDummyPrivateKey.
func Parse(r io.Reader) (f func(out, in []byte), err error) {
	params, err := ParseIntoParams(r)
	if err != nil {
		return nil, err
	}

	return params.Function()
}

// ParseIntoParams reads a binary specification for a string-to-key
// transformation from r and returns a struct describing the s2k parameters.
func ParseIntoParams(r io.Reader) (params *Params, err error) {
	var buf [Argon2SaltSize + 3]byte

	_, err = io.ReadFull(r, buf[:1])
	if err != nil {
		return
	}

	params = &Params{
		mode: Mode(buf[0]),
	}

	switch params.mode {
	case SimpleS2K:
		_, err = io.ReadFull(r, buf[:1])
		if err != nil {
			return nil, err
		}
		params.hashId = buf[0]
		return params, nil
	case SaltedS2K:
		_, err = io.ReadFull(r, buf[:9])
		if err != nil {
			return nil, err
		}
		params.hashId = buf[0]
		params.salt = buf[1:9]
		return params, nil
	case IteratedSaltedS2K:
		_, err = io.ReadFull(r, buf[:10])
		if err != nil {
			return nil, err
		}
		params.hashId = buf[0]
		params.salt = buf[1:9]
		params.countByte = buf[9]
		return params, nil
	case Argon2S2K:
		_, err = io.ReadFull(r, buf[:Argon2SaltSize+3])
		if err != nil {
			return nil, err
		}
		params.salt = buf[:Argon2SaltSize]
		params.passes = buf[Argon2SaltSize]
		params.parallelism = buf[Argon2SaltSize+1]
		params.memoryExp = buf[Argon2SaltSize+2]
		return params, nil
	case GnuS2K:
		// This is a GNU extension. See
		// https://git.gnupg.org/cgi-bin/gitweb.cgi?p=gnupg.git;a=blob;f=doc/DETAILS;h=fe55ae16ab4e26d8356dc574c9e8bc935e71aef1;hb=23191d7851eae2217ecdac6484349849a24fd94a#l1109
		if _, err = io.ReadFull(r, buf[:5]); err != nil {
			return nil, err
		}
		params.hashId = buf[0]
		if buf[1] == 'G' && buf[2] == 'N' && buf[3] == 'U' && buf[4] == 1 {
			return params, nil
		}
		return nil, errors.UnsupportedError("GNU S2K extension")
	}

	return nil, errors.UnsupportedError("S2K function")
}

func (params *Params) Dummy() bool {
	return params != nil && params.mode == GnuS2K
}

func (params *Params) Function() (f func(out, in []byte), err error) {
	if params.Dummy() {
		return nil, errors.ErrDummyPrivateKey("dummy key found")
	}
	var hashObj crypto.Hash
	if params.mode != Argon2S2K {
		var ok bool
		hashObj, ok = algorithm.HashIdToHashWithSha1(params.hashId)
		if !ok {
			return nil, errors.UnsupportedError("hash for S2K function: " + strconv.Itoa(int(params.hashId)))
		}
		if !hashObj.Available() {
			return nil, errors.UnsupportedError("hash not available: " + strconv.Itoa(int(hashObj)))
		}
	}

	switch params.mode {
	case SimpleS2K:
		f := func(out, in []byte) {
			Simple(out, hashObj.New(), in)
		}

		return f, nil
	case SaltedS2K:
		f := func(out, in []byte) {
			Salted(out, hashObj.New(), in, params.salt)
		}

		return f, nil
	case IteratedSaltedS2K:
		f := func(out, in []byte) {
			Iterated(out, hashObj.New(), in, params.salt, decodeCount(params.countByte))
		}

		return f, nil
	case Argon2S2K:
		f := func(out, in []byte) {
			Argon2(out, in, params.salt, params.passes, params.parallelism, params.memoryExp)
		}
		return f, nil
	}

	return nil, errors.UnsupportedError("S2K function")
}

func (params *Params) Serialize(w io.Writer) (err error) {
	if _, err = w.Write([]byte{uint8(params.mode)}); err != nil {
		return
	}
	if params.mode != Argon2S2K {
		if _, err = w.Write([]byte{params.hashId}); err != nil {
			return
		}
	}
	if params.Dummy() {
		_, err = w.Write(append([]byte("GNU"), 1))
		return
	}
	if params.mode > 0 {
		if _, err = w.Write(params.salt); err != nil {
			return
		}
		if params.mode == IteratedSaltedS2K {
			_, err = w.Write([]byte{params.countByte})
		}
		if params.mode == Argon2S2K {
			_, err = w.Write([]byte{params.passes, params.parallelism, params.memoryExp})
		}
	}
	return
}

// Serialize salts and stretches the given passphrase and writes the
// resulting key into key. It also serializes an S2K descriptor to
// w. The key stretching can be configured with c, which may be
// nil. In that case, sensible defaults will be used.
func Serialize(w io.Writer, key []byte, rand io.Reader, passphrase []byte, c *Config) error {
	params, err := Generate(rand, c)
	if err != nil {
		return err
	}
	err = params.Serialize(w)
	if err != nil {
		return err
	}

	f, err := params.Function()
	if err != nil {
		return err
	}
	f(key, passphrase)
	return nil
}
