package main

import (
    "time"

    log "github.com/sirupsen/logrus"
)

// Interplanetary protocol implementation, crypto and protobuf

type InterplanetaryProtocol struct {
    crypto *InterplanetaryCrypto
}

func NewInterplanetaryProtocol(crypto *InterplanetaryCrypto) *InterplanetaryProtocol {
    return &InterplanetaryProtocol {
        crypto: crypto,
    }
}

func (ipp *InterplanetaryProtocol) NewInterplanetaryMessage(content string) []byte {
    if ipp.crypto != nil { // If crypto is inited, apply encryption
        return ipp.newEncryptedInterplanetaryMessage(content)
    } else {
        return ipp.newPlaintextInterplanetaryMessage(content)
    }
}

func (ipp *InterplanetaryProtocol) newEncryptedInterplanetaryMessage(content string) []byte {
    now := time.Now().Unix()
    textMessage := &TextMessage {
        Created: &now,
        Text: &content,
    }

    // Marshal into basic text message format
    plain_bytes, err := textMessage.Marshal()
    if err != nil {
        log.WithError(err).Error("Could not marshal plaintext message.")
        return nil
    }

    // and encrypt it
    unpadded_len := int64(len(plain_bytes))
    ciphertext, iv, message_key_ct := ipp.crypto.EncryptMessage(plain_bytes)

    // Embed encrypted key, iv and message itself in protobuf
    interplantery_message := &InterplanetaryMessage {
        Type: InterplanetaryMessage_ENCRYPTED_TEXT_MESSAGE.Enum(),
        EncryptedTextMessage: &EncryptedTextMessage {
            Iv: iv,
            MessageKey: message_key_ct,
            UnpaddedLen: &unpadded_len,
            Data: ciphertext,
        },
    }

    // and marshal
    msg_bytes, err := interplantery_message.Marshal()
    if err != nil {
        log.WithError(err).Error("Could not marshal message.")
        return nil
    }

    return msg_bytes
}

func (ipp *InterplanetaryProtocol) newPlaintextInterplanetaryMessage(content string) []byte {
    now := time.Now().Unix()

    // Simple unencrypted text message, just marshal and return
    interplantery_message := &InterplanetaryMessage {
        Type: InterplanetaryMessage_TEXT_MESSAGE.Enum(),
        TextMessage: &TextMessage {
            Created: &now,
            Text: &content,
        },
    }

    msg_bytes, err := interplantery_message.Marshal()
    if err != nil {
        log.WithError(err).Error("Could not marshal message.")
        return nil
    }

    return msg_bytes
}

var encrypted_placeholder string = "ENCRYPTED"
func (ipp *InterplanetaryProtocol) ProcessInterplanetaryMessage(msg_bytes []byte) *TextMessage {
    // Try to parse message
    interplantery_message := &InterplanetaryMessage{}
    err := interplantery_message.Unmarshal(msg_bytes)
    if err != nil {
        log.WithError(err).Error("Could not unmarshal message.")
        return nil
    }

    processed_msg := &TextMessage{}
    switch *interplantery_message.Type {
    case InterplanetaryMessage_TEXT_MESSAGE:
        // Plaintext, just unmarshal
        processed_msg = interplantery_message.TextMessage
    case InterplanetaryMessage_ENCRYPTED_TEXT_MESSAGE:
        // Encrypted, attempt to decrypt if crypto is inited
        log.Info("Received encrypted message.")
        if ipp.crypto == nil {
            // If not, print out placeholder.
            processed_msg = &TextMessage {
                Text: &encrypted_placeholder,
            }
        } else {
            // Attempt to decrypt
            encrypted_msg := interplantery_message.EncryptedTextMessage
            decrypted_msg_bytes := ipp.crypto.DecryptMessage(
                        encrypted_msg.MessageKey,
                        encrypted_msg.Iv,
                        encrypted_msg.Data,
                        *encrypted_msg.UnpaddedLen)
            err := processed_msg.Unmarshal(decrypted_msg_bytes)
            if err != nil {
                log.WithError(err).Error("Could not unmarshal decrypted message.")
                return nil
            }
        }
    }

    log.WithFields(log.Fields {
        "created": processed_msg.Created,
        "text": processed_msg.Text,
    }).Debug("Processed incoming message.")

    return processed_msg
}

