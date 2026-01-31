package negotiate

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"errors"
	"fmt"
)

// Wire format magic bytes and version
const (
	clientHelloMagic = 0x41584348 // "AXCH" in big-endian
	serverHelloMagic = 0x41585348 // "AXSH" in big-endian
	wireVersion      = 1
)

// Wire format errors
var (
	ErrInvalidMagic   = errors.New("negotiate: invalid magic bytes")
	ErrInvalidWireVer = errors.New("negotiate: unsupported wire version")
	ErrDataTooShort   = errors.New("negotiate: data too short")
	ErrInvalidLength  = errors.New("negotiate: invalid field length")
)

// WireClientHello is the wire format for ClientHello.
type WireClientHello struct {
	Version      uint32
	SenderDID    string
	Profiles     []Profile
	Capabilities Capabilities
	TimestampMs  uint64
	Signature    []byte
}

// WireServerHello is the wire format for ServerHello.
type WireServerHello struct {
	Version         uint32
	SenderDID       string
	SelectedProfile Profile
	Capabilities    Capabilities
	TimestampMs     uint64
	Signature       []byte
}

// Marshal serializes the ClientHello to wire format.
func (h *WireClientHello) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Magic (4 bytes)
	if err := binary.Write(buf, binary.BigEndian, uint32(clientHelloMagic)); err != nil {
		return nil, err
	}

	// Wire version (1 byte)
	buf.WriteByte(wireVersion)

	// Protocol version (4 bytes)
	if err := binary.Write(buf, binary.BigEndian, h.Version); err != nil {
		return nil, err
	}

	// SenderDID (length-prefixed string)
	if err := writeString(buf, h.SenderDID); err != nil {
		return nil, err
	}

	// Profiles count (1 byte) + profiles
	if len(h.Profiles) > 255 {
		return nil, errors.New("negotiate: too many profiles")
	}
	buf.WriteByte(byte(len(h.Profiles)))
	for _, p := range h.Profiles {
		if err := writeString(buf, string(p)); err != nil {
			return nil, err
		}
	}

	// Capabilities
	if err := writeCapabilities(buf, h.Capabilities); err != nil {
		return nil, err
	}

	// Timestamp (8 bytes)
	if err := binary.Write(buf, binary.BigEndian, h.TimestampMs); err != nil {
		return nil, err
	}

	// Signature (length-prefixed bytes)
	if err := writeBytes(buf, h.Signature); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Marshal serializes the ServerHello to wire format.
func (h *WireServerHello) Marshal() ([]byte, error) {
	buf := new(bytes.Buffer)

	// Magic (4 bytes)
	if err := binary.Write(buf, binary.BigEndian, uint32(serverHelloMagic)); err != nil {
		return nil, err
	}

	// Wire version (1 byte)
	buf.WriteByte(wireVersion)

	// Protocol version (4 bytes)
	if err := binary.Write(buf, binary.BigEndian, h.Version); err != nil {
		return nil, err
	}

	// SenderDID (length-prefixed string)
	if err := writeString(buf, h.SenderDID); err != nil {
		return nil, err
	}

	// SelectedProfile (length-prefixed string)
	if err := writeString(buf, string(h.SelectedProfile)); err != nil {
		return nil, err
	}

	// Capabilities
	if err := writeCapabilities(buf, h.Capabilities); err != nil {
		return nil, err
	}

	// Timestamp (8 bytes)
	if err := binary.Write(buf, binary.BigEndian, h.TimestampMs); err != nil {
		return nil, err
	}

	// Signature (length-prefixed bytes)
	if err := writeBytes(buf, h.Signature); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalClientHello deserializes a ClientHello from wire format.
func UnmarshalClientHello(data []byte) (*WireClientHello, error) {
	if len(data) < 9 { // magic(4) + wireVer(1) + version(4)
		return nil, ErrDataTooShort
	}

	buf := bytes.NewReader(data)
	h := &WireClientHello{}

	// Magic
	var magic uint32
	if err := binary.Read(buf, binary.BigEndian, &magic); err != nil {
		return nil, err
	}
	if magic != clientHelloMagic {
		return nil, ErrInvalidMagic
	}

	// Wire version
	wireVer, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if wireVer != wireVersion {
		return nil, ErrInvalidWireVer
	}

	// Protocol version
	if err := binary.Read(buf, binary.BigEndian, &h.Version); err != nil {
		return nil, err
	}

	// SenderDID
	h.SenderDID, err = readString(buf)
	if err != nil {
		return nil, err
	}

	// Profiles
	numProfiles, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	h.Profiles = make([]Profile, numProfiles)
	for i := range h.Profiles {
		s, err := readString(buf)
		if err != nil {
			return nil, err
		}
		h.Profiles[i] = Profile(s)
	}

	// Capabilities
	h.Capabilities, err = readCapabilities(buf)
	if err != nil {
		return nil, err
	}

	// Timestamp
	if err := binary.Read(buf, binary.BigEndian, &h.TimestampMs); err != nil {
		return nil, err
	}

	// Signature
	h.Signature, err = readBytes(buf)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// UnmarshalServerHello deserializes a ServerHello from wire format.
func UnmarshalServerHello(data []byte) (*WireServerHello, error) {
	if len(data) < 9 { // magic(4) + wireVer(1) + version(4)
		return nil, ErrDataTooShort
	}

	buf := bytes.NewReader(data)
	h := &WireServerHello{}

	// Magic
	var magic uint32
	if err := binary.Read(buf, binary.BigEndian, &magic); err != nil {
		return nil, err
	}
	if magic != serverHelloMagic {
		return nil, ErrInvalidMagic
	}

	// Wire version
	wireVer, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	if wireVer != wireVersion {
		return nil, ErrInvalidWireVer
	}

	// Protocol version
	if err := binary.Read(buf, binary.BigEndian, &h.Version); err != nil {
		return nil, err
	}

	// SenderDID
	h.SenderDID, err = readString(buf)
	if err != nil {
		return nil, err
	}

	// SelectedProfile
	profileStr, err := readString(buf)
	if err != nil {
		return nil, err
	}
	h.SelectedProfile = Profile(profileStr)

	// Capabilities
	h.Capabilities, err = readCapabilities(buf)
	if err != nil {
		return nil, err
	}

	// Timestamp
	if err := binary.Read(buf, binary.BigEndian, &h.TimestampMs); err != nil {
		return nil, err
	}

	// Signature
	h.Signature, err = readBytes(buf)
	if err != nil {
		return nil, err
	}

	return h, nil
}

// Sign signs the ClientHello with the given private key.
func (h *WireClientHello) Sign(privateKey ed25519.PrivateKey) error {
	// Build transcript for signing (everything except signature)
	transcript := h.buildTranscript()
	h.Signature = ed25519.Sign(privateKey, transcript)
	return nil
}

// Verify verifies the ClientHello signature with the given public key.
func (h *WireClientHello) Verify(publicKey ed25519.PublicKey) error {
	transcript := h.buildTranscript()
	if !ed25519.Verify(publicKey, transcript, h.Signature) {
		return ErrSignatureVerificationFailed
	}
	return nil
}

// buildTranscript builds the data to be signed for ClientHello.
func (h *WireClientHello) buildTranscript() []byte {
	buf := new(bytes.Buffer)

	// Context string
	buf.WriteString("AXCP-ClientHello-v1")

	// Version
	binary.Write(buf, binary.BigEndian, h.Version)

	// SenderDID
	writeString(buf, h.SenderDID)

	// Profiles
	buf.WriteByte(byte(len(h.Profiles)))
	for _, p := range h.Profiles {
		writeString(buf, string(p))
	}

	// Capabilities
	writeCapabilities(buf, h.Capabilities)

	// Timestamp
	binary.Write(buf, binary.BigEndian, h.TimestampMs)

	return buf.Bytes()
}

// Sign signs the ServerHello with the given private key.
func (h *WireServerHello) Sign(privateKey ed25519.PrivateKey) error {
	transcript := h.buildTranscript()
	h.Signature = ed25519.Sign(privateKey, transcript)
	return nil
}

// Verify verifies the ServerHello signature with the given public key.
func (h *WireServerHello) Verify(publicKey ed25519.PublicKey) error {
	transcript := h.buildTranscript()
	if !ed25519.Verify(publicKey, transcript, h.Signature) {
		return ErrSignatureVerificationFailed
	}
	return nil
}

// buildTranscript builds the data to be signed for ServerHello.
func (h *WireServerHello) buildTranscript() []byte {
	buf := new(bytes.Buffer)

	// Context string
	buf.WriteString("AXCP-ServerHello-v1")

	// Version
	binary.Write(buf, binary.BigEndian, h.Version)

	// SenderDID
	writeString(buf, h.SenderDID)

	// SelectedProfile
	writeString(buf, string(h.SelectedProfile))

	// Capabilities
	writeCapabilities(buf, h.Capabilities)

	// Timestamp
	binary.Write(buf, binary.BigEndian, h.TimestampMs)

	return buf.Bytes()
}

// Helper functions for wire format

func writeString(buf *bytes.Buffer, s string) error {
	b := []byte(s)
	if len(b) > 65535 {
		return ErrInvalidLength
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(len(b))); err != nil {
		return err
	}
	_, err := buf.Write(b)
	return err
}

func readString(buf *bytes.Reader) (string, error) {
	var length uint16
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return "", err
	}
	b := make([]byte, length)
	if _, err := buf.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func writeBytes(buf *bytes.Buffer, b []byte) error {
	if len(b) > 65535 {
		return ErrInvalidLength
	}
	if err := binary.Write(buf, binary.BigEndian, uint16(len(b))); err != nil {
		return err
	}
	_, err := buf.Write(b)
	return err
}

func readBytes(buf *bytes.Reader) ([]byte, error) {
	var length uint16
	if err := binary.Read(buf, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	b := make([]byte, length)
	if _, err := buf.Read(b); err != nil {
		return nil, err
	}
	return b, nil
}

func writeCapabilities(buf *bytes.Buffer, cap Capabilities) error {
	// Flags byte: bit 0 = RequireAuth, bit 1 = RequireReplay
	var flags byte
	if cap.RequireAuth {
		flags |= 1
	}
	if cap.RequireReplay {
		flags |= 2
	}
	buf.WriteByte(flags)

	// SigAlgos count (1 byte) + algos
	if len(cap.SigAlgos) > 255 {
		return fmt.Errorf("negotiate: too many signature algorithms")
	}
	buf.WriteByte(byte(len(cap.SigAlgos)))
	for _, algo := range cap.SigAlgos {
		if err := writeString(buf, algo); err != nil {
			return err
		}
	}

	return nil
}

func readCapabilities(buf *bytes.Reader) (Capabilities, error) {
	cap := Capabilities{}

	// Flags byte
	flags, err := buf.ReadByte()
	if err != nil {
		return cap, err
	}
	cap.RequireAuth = (flags & 1) != 0
	cap.RequireReplay = (flags & 2) != 0

	// SigAlgos
	numAlgos, err := buf.ReadByte()
	if err != nil {
		return cap, err
	}
	cap.SigAlgos = make([]string, numAlgos)
	for i := range cap.SigAlgos {
		cap.SigAlgos[i], err = readString(buf)
		if err != nil {
			return cap, err
		}
	}

	return cap, nil
}
