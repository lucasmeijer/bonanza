package crypto

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"

	"filippo.io/edwards25519"

	"github.com/buildbarn/bb-storage/pkg/util"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PublicKeyToECDHPublicKey converts an arbitrary public key type (e.g.,
// one embedded in an X.509 certificate) to an instance of
// *ecdh.PublicKey, so that ECDH can be performed.
func PublicKeyToECDHPublicKey(publicKey any) (*ecdh.PublicKey, error) {
	switch publicKey := publicKey.(type) {
	case *ecdsa.PublicKey:
		ecdhPublicKey, err := publicKey.ECDH()
		if err != nil {
			return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Failed to create ECDH public key from ECDSA public key")
		}
		return ecdhPublicKey, nil
	case *ecdh.PublicKey:
		return publicKey, nil
	case ed25519.PublicKey:
		// As X25519 cannot be used for signing, it's not
		// possible to create Certificate Signing Requests
		// (CSRs) for them, making them hard to use in
		// combination with client certificates. We therefore
		// also permit public keys of type Ed25519, which we
		// convert to a X25519 public key using the birational
		// map provided in RFC 7748.
		var point edwards25519.Point
		if _, err := point.SetBytes(publicKey); err != nil {
			return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid Ed25519 public key")
		}
		var err error
		ecdhPublicKey, err := ecdh.X25519().NewPublicKey(point.BytesMontgomery())
		if err != nil {
			return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Failed to create X25519 public key from Ed25519 public key")
		}
		return ecdhPublicKey, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "Public key is not of type ECDH, ECDSA or Ed25519")
	}
}

// ParsePKIXECDHPublicKey parses a public key that is encoded in PKIX
// format to an instance of *ecdh.PublicKey.
func ParsePKIXECDHPublicKey(pkixPublicKey []byte) (*ecdh.PublicKey, error) {
	parsedPublicKey, err := x509.ParsePKIXPublicKey(pkixPublicKey)
	if err != nil {
		return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Invalid PKIX public key")
	}
	return PublicKeyToECDHPublicKey(parsedPublicKey)
}

// ParsePEMWithPKCS8ECDHPrivateKey parses a PCKS #8 encoded ECDH, ECDSA
// or Ed25519 private key. All of them are converted to an instance of
// *ecdh.PrivateKey, so that Elliptic Curve Diffie-Hellman can be
// performed on them.
func ParsePEMWithPKCS8ECDHPrivateKey(data []byte) (*ecdh.PrivateKey, error) {
	privateKeyBlock, _ := pem.Decode(data)
	if privateKeyBlock == nil {
		return nil, status.Error(codes.InvalidArgument, "Private key does not contain a PEM block")
	}
	if privateKeyBlock.Type != "PRIVATE KEY" {
		return nil, status.Error(codes.InvalidArgument, "Private key PEM block is not of type PRIVATE KEY")
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(privateKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}
	switch privateKey := privateKey.(type) {
	case *ecdh.PrivateKey:
		return privateKey, nil
	case *ecdsa.PrivateKey:
		ecdhPrivateKey, err := privateKey.ECDH()
		if err != nil {
			return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Failed to create ECDH private key from ECDSA private key")
		}
		return ecdhPrivateKey, nil
	case ed25519.PrivateKey:
		seedHash := sha512.Sum512(privateKey.Seed())
		ecdhPrivateKey, err := ecdh.X25519().NewPrivateKey(seedHash[:32])
		if err != nil {
			return nil, util.StatusWrapWithCode(err, codes.InvalidArgument, "Failed to create X25519 private key from Ed25519 private key")
		}
		return ecdhPrivateKey, nil
	default:
		return nil, status.Error(codes.InvalidArgument, "Private key is not of type ECDH, ECDSA or Ed25519")
	}
}
