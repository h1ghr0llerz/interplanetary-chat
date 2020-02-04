package main

import (
    "crypto/aes"
    "crypto/sha1"
    "io"

    "golang.org/x/crypto/pbkdf2"
    "github.com/MidnightWonderer/IGE-go/ige"
    log "github.com/sirupsen/logrus"
)

type InterplanetaryCrypto struct {
    master_key []byte
    randReader io.Reader
}

const BLOCK_SIZE = 32

func pad_to_block_size(bb []byte, block_size int) []byte {
    l := len(bb)
    pad_length := l + (block_size - (l % block_size))

    tmp := make([]byte, pad_length)
    copy(tmp[:], bb)
    return tmp
}

func xor_bytes(dst, a, b []byte, n int) {
    for i := 0; i < n; i++ {
        dst[i] = a[i] ^ b[i]
    }
}

func NewInterplanetaryCrypto(passphrase string, saltphrase string, randReader io.Reader) *InterplanetaryCrypto {
    salt_hash := sha1.Sum([]byte(saltphrase))
    derived_key := pbkdf2.Key([]byte(passphrase), salt_hash[:], 4096, BLOCK_SIZE, sha1.New) // 32 byte key

    ic := &InterplanetaryCrypto {
        master_key: derived_key,
        randReader: randReader,
    }

    return ic
}

func (ic *InterplanetaryCrypto) DecryptMessage(message_key_ct, iv, data []byte, unpadded_length int64) []byte {
    plaintext := make([]byte, len(data))
    copy(plaintext, data)

    // Decrypt message_key using master key
    message_key := make([]byte, len(message_key_ct))
    xor_bytes(message_key, ic.master_key, message_key_ct, len(message_key_ct))

    block, err := aes.NewCipher(message_key)
    if err != nil {
        log.WithError(err).Fatal("Could not initialize crypto engine for message")
        return nil
    }
    ige_decrypter := ige.NewIGEDecrypter(block, iv)

    ige_decrypter.CryptBlocks(plaintext, plaintext)
    return plaintext[:unpadded_length]
}

func (ic *InterplanetaryCrypto) EncryptMessage(data []byte) (ciphertext, iv, message_key_ct []byte) {
    data_padded := pad_to_block_size(data, BLOCK_SIZE) // block size
    ciphertext = make([]byte, len(data_padded))
    copy(ciphertext, data_padded)

    // Make random IV and message key, ratcheting schmacheting...
    message_key := make([]byte, BLOCK_SIZE)
    iv = make([]byte, BLOCK_SIZE)

    ic.randReader.Read(iv)
    ic.randReader.Read(message_key)

    // Encrypt message_key using master key
    message_key_ct = make([]byte, BLOCK_SIZE)
    xor_bytes(message_key_ct, ic.master_key, message_key, len(message_key))

    // Encrypt rest of message using message key, with AES-IGE
    block, err := aes.NewCipher(message_key)
    if err != nil {
        log.WithError(err).Fatal("Could not initialize crypto engine for message")
        return nil, nil, nil
    }
    ige_encrypter := ige.NewIGEEncrypter(block, iv)

    ige_encrypter.CryptBlocks(ciphertext, ciphertext) // in place
    return
}

