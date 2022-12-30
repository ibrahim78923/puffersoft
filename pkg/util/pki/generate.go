/*
Copyright 2020 The cert-manager Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	v1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
)

const (
	// MinRSAKeySize is the minimum RSA keysize allowed to be generated by the
	// generator functions in this package.
	MinRSAKeySize = 2048

	// MaxRSAKeySize is the maximum RSA keysize allowed to be generated by the
	// generator functions in this package.
	MaxRSAKeySize = 8192

	// ECCurve256 represents a secp256r1 / prime256v1 / NIST P-256 ECDSA key.
	ECCurve256 = 256
	// ECCurve384 represents a secp384r1 / NIST P-384 ECDSA key.
	ECCurve384 = 384
	// ECCurve521 represents a secp521r1 / NIST P-521 ECDSA key.
	ECCurve521 = 521
)

// GeneratePrivateKeyForCertificate will generate a private key suitable for
// the provided cert-manager Certificate resource, taking into account the
// parameters on the provided resource.
// The returned key will either be RSA or ECDSA.
func GeneratePrivateKeyForCertificate(crt *v1.Certificate) (crypto.Signer, error) {
	crt = crt.DeepCopy()
	if crt.Spec.PrivateKey == nil {
		crt.Spec.PrivateKey = &v1.CertificatePrivateKey{}
	}
	switch crt.Spec.PrivateKey.Algorithm {
	case v1.PrivateKeyAlgorithm(""), v1.RSAKeyAlgorithm:
		keySize := MinRSAKeySize

		if crt.Spec.PrivateKey.Size > 0 {
			keySize = crt.Spec.PrivateKey.Size
		}

		return GenerateRSAPrivateKey(keySize)
	case v1.ECDSAKeyAlgorithm:
		keySize := ECCurve256

		if crt.Spec.PrivateKey.Size > 0 {
			keySize = crt.Spec.PrivateKey.Size
		}

		return GenerateECPrivateKey(keySize)
	case v1.Ed25519KeyAlgorithm:
		return GenerateEd25519PrivateKey()
	default:
		return nil, fmt.Errorf("unsupported private key algorithm specified: %s", crt.Spec.PrivateKey.Algorithm)
	}
}

// GenerateRSAPrivateKey will generate a RSA private key of the given size.
// It places restrictions on the minimum and maximum RSA keysize.
func GenerateRSAPrivateKey(keySize int) (*rsa.PrivateKey, error) {
	// Do not allow keySize < 2048
	// https://en.wikipedia.org/wiki/Key_size#cite_note-twirl-14
	if keySize < MinRSAKeySize {
		return nil, fmt.Errorf("weak rsa key size specified: %d. minimum key size: %d", keySize, MinRSAKeySize)
	}
	if keySize > MaxRSAKeySize {
		return nil, fmt.Errorf("rsa key size specified too big: %d. maximum key size: %d", keySize, MaxRSAKeySize)
	}

	return rsa.GenerateKey(rand.Reader, keySize)
}

// GenerateECPrivateKey will generate an ECDSA private key of the given size.
// It can be used to generate 256, 384 and 521 sized keys.
func GenerateECPrivateKey(keySize int) (*ecdsa.PrivateKey, error) {
	var ecCurve elliptic.Curve

	switch keySize {
	case ECCurve256:
		ecCurve = elliptic.P256()
	case ECCurve384:
		ecCurve = elliptic.P384()
	case ECCurve521:
		ecCurve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported ecdsa key size specified: %d", keySize)
	}

	return ecdsa.GenerateKey(ecCurve, rand.Reader)
}

// GenerateEd25519PrivateKey will generate an Ed25519 private key
func GenerateEd25519PrivateKey() (ed25519.PrivateKey, error) {

	_, prvkey, err := ed25519.GenerateKey(rand.Reader)

	return prvkey, err
}

// EncodePrivateKey will encode a given crypto.PrivateKey by first inspecting
// the type of key encoding and then inspecting the type of key provided.
// It only supports encoding RSA or ECDSA keys.
func EncodePrivateKey(pk crypto.PrivateKey, keyEncoding v1.PrivateKeyEncoding) ([]byte, error) {
	switch keyEncoding {
	case v1.PrivateKeyEncoding(""), v1.PKCS1:
		switch k := pk.(type) {
		case *rsa.PrivateKey:
			return EncodePKCS1PrivateKey(k), nil
		case *ecdsa.PrivateKey:
			return EncodeECPrivateKey(k)
		case ed25519.PrivateKey:
			return EncodePKCS8PrivateKey(k)
		default:
			return nil, fmt.Errorf("error encoding private key: unknown key type: %T", pk)
		}
	case v1.PKCS8:
		return EncodePKCS8PrivateKey(pk)
	default:
		return nil, fmt.Errorf("error encoding private key: unknown key encoding: %s", keyEncoding)
	}
}

// EncodePKCS1PrivateKey will marshal a RSA private key into x509 PEM format.
func EncodePKCS1PrivateKey(pk *rsa.PrivateKey) []byte {
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(pk)}

	return pem.EncodeToMemory(block)
}

// EncodePKCS8PrivateKey will marshal a private key into x509 PEM format.
func EncodePKCS8PrivateKey(pk interface{}) ([]byte, error) {
	keyBytes, err := x509.MarshalPKCS8PrivateKey(pk)
	if err != nil {
		return nil, err
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}

	return pem.EncodeToMemory(block), nil
}

// EncodeECPrivateKey will marshal an ECDSA private key into x509 PEM format.
func EncodeECPrivateKey(pk *ecdsa.PrivateKey) ([]byte, error) {
	asnBytes, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return nil, fmt.Errorf("error encoding private key: %s", err.Error())
	}

	block := &pem.Block{Type: "EC PRIVATE KEY", Bytes: asnBytes}
	return pem.EncodeToMemory(block), nil
}

// PublicKeyForPrivateKey will return the crypto.PublicKey for the given
// crypto.PrivateKey. It only supports RSA and ECDSA keys.
func PublicKeyForPrivateKey(pk crypto.PrivateKey) (crypto.PublicKey, error) {
	switch k := pk.(type) {
	case *rsa.PrivateKey:
		return k.Public(), nil
	case *ecdsa.PrivateKey:
		return k.Public(), nil
	case ed25519.PrivateKey:
		return k.Public(), nil
	default:
		return nil, fmt.Errorf("unknown private key type: %T", pk)
	}
}

// PublicKeyMatchesCertificate checks whether the given public key matches the
// public key in the given x509.Certificate.
// Returns false and no error if the public key is *not* the same as the certificate's key
// Returns true and no error if the public key *is* the same as the certificate's key
// Returns an error if the certificate's key type cannot be determined (i.e. non RSA/ECDSA keys)
func PublicKeyMatchesCertificate(check crypto.PublicKey, crt *x509.Certificate) (bool, error) {
	return PublicKeysEqual(crt.PublicKey, check)
}

// PublicKeyMatchesCSR can be used to verify the given public key matches the
// public key in the given x509.CertificateRequest.
// Returns false and no error if the given public key is *not* the same as the CSR's key
// Returns true and no error if the given public key *is* the same as the CSR's key
// Returns an error if the CSR's key type cannot be determined (i.e. non RSA/ECDSA keys)
func PublicKeyMatchesCSR(check crypto.PublicKey, csr *x509.CertificateRequest) (bool, error) {
	return PublicKeysEqual(csr.PublicKey, check)
}

// PublicKeysEqual compares two given public keys for equality.
// The definition of "equality" depends on the type of the public keys.
// Returns true if the keys are the same, false if they differ or an error if
// the key type of `a` cannot be determined.
func PublicKeysEqual(a, b crypto.PublicKey) (bool, error) {
	switch pub := a.(type) {
	case *rsa.PublicKey:
		return pub.Equal(b), nil
	case *ecdsa.PublicKey:
		return pub.Equal(b), nil
	case ed25519.PublicKey:
		return pub.Equal(b), nil
	default:
		return false, fmt.Errorf("unrecognised public key type: %T", a)
	}
}
