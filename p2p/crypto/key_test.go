package crypto_test

import (
	. "github.com/jbenet/go-ipfs/p2p/crypto"

	"bytes"
	u "github.com/jbenet/go-ipfs/util"
	tu "github.com/jbenet/go-ipfs/util/testutil"
	"testing"
)

func TestRsaKeys(t *testing.T) {
	sk, pk, err := tu.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}
	testKey(t, sk, pk)
}

func TestEd25519Keys(t *testing.T) {
	sk, pk, err := GenerateKeyPairWithReader(Ed25519, 0, u.NewTimeSeededRand())
	if err != nil {
		t.Fatal(err)
	}
	testKey(t, sk, pk)
}

func testKey(t *testing.T, sk PrivKey, pk PubKey) {
	testKeySignature(t, sk)
	testKeyEncoding(t, sk)
	testKeyEquals(t, sk)
	testKeyEquals(t, pk)
}

func testKeySignature(t *testing.T, sk PrivKey) {
	pk := sk.GetPublic()

	text, err := GenSecret()
	if err != nil {
		t.Fatal(err)
	}

	sig, err := sk.Sign(text)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := pk.Verify(text, sig)
	if err != nil {
		t.Fatal(err)
	}

	if !valid {
		t.Fatal("Invalid signature.")
	}
}

func testKeyEncoding(t *testing.T, sk PrivKey) {
	skbm, err := MarshalPrivateKey(sk)
	if err != nil {
		t.Fatal(err)
	}

	sk2, err := UnmarshalPrivateKey(skbm)
	if err != nil {
		t.Fatal(err)
	}

	skbm2, err := MarshalPrivateKey(sk2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(skbm, skbm2) {
		t.Error("skb -> marshal -> unmarshal -> skb failed.\n", skbm, "\n", skbm2)
	}

	pk := sk.GetPublic()
	pkbm, err := MarshalPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}

	_, err = UnmarshalPublicKey(pkbm)
	if err != nil {
		t.Fatal(err)
	}

	pkbm2, err := MarshalPublicKey(pk)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pkbm, pkbm2) {
		t.Error("skb -> marshal -> unmarshal -> skb failed.\n", pkbm, "\n", pkbm2)
	}
}

func testKeyEquals(t *testing.T, k Key) {
	kb, err := k.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	if !KeyEqual(k, k) {
		t.Fatal("Key not equal to itself.")
	}

	if !KeyEqual(k, testkey(kb)) {
		t.Fatal("Key not equal to key with same bytes.")
	}

	sk, pk, err := tu.RandTestKeyPair(512)
	if err != nil {
		t.Fatal(err)
	}

	if KeyEqual(k, sk) {
		t.Fatal("Keys should not equal.")
	}

	if KeyEqual(k, pk) {
		t.Fatal("Keys should not equal.")
	}
}

type testkey []byte

func (pk testkey) Bytes() ([]byte, error) {
	return pk, nil
}

func (pk testkey) Equals(k Key) bool {
	return KeyEqual(pk, k)
}

func (pk testkey) Hash() ([]byte, error) {
	return KeyHash(pk)
}
