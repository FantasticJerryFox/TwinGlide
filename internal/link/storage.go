package link

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type Shared struct {
	SourceID      string `json:"sourceID"`
	Side          string `json:"side"`
	Enabled       bool   `json:"enabled"`
	Revision      int64  `json:"revision"`
	Actor         string `json:"actor"`
	EdgeDelayMS   int    `json:"edgeDelayMS"`
	ReturnKey     string `json:"returnKey"`
	AudioTarget   string `json:"audioTarget"`
	AudioVolume   int    `json:"audioVolume"`
	AudioBufferMS int    `json:"audioBufferMS"`
}
type Preferences struct {
	Theme        string `json:"theme"`
	StartAtLogin bool   `json:"startAtLogin"`
}
type Peer struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	OS      string `json:"os"`
	Address string `json:"address"`
}
type Config struct {
	Version     int         `json:"version"`
	Name        string      `json:"name"`
	Peer        *Peer       `json:"peer,omitempty"`
	Shared      Shared      `json:"shared"`
	Preferences Preferences `json:"preferences"`
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(name, path)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(b, '\n'))
}
func cleanName(s string) string {
	var b strings.Builder
	count := 0
	for _, r := range s {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
			count++
		}
		if count >= 80 {
			break
		}
	}
	v := strings.TrimSpace(b.String())
	if v == "" {
		return "Computer"
	}
	return v
}
func certificateID(raw []byte) string { h := sha256.Sum256(raw); return hex.EncodeToString(h[:]) }
func validID(id string) bool {
	b, e := hex.DecodeString(id)
	return e == nil && len(b) == 32 && id == strings.ToLower(id)
}
func readIdentity(dir string) (tls.Certificate, error) {
	path := filepath.Join(dir, "identity.pem")
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		key, e := rsa.GenerateKey(rand.Reader, 3072)
		if e != nil {
			return tls.Certificate{}, e
		}
		serial, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if e != nil {
			return tls.Certificate{}, e
		}
		cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "TwinGlide device"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().AddDate(10, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
		der, e := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
		if e != nil {
			return tls.Certificate{}, e
		}
		raw = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})...)
		if e = atomicWrite(path, raw); e != nil {
			return tls.Certificate{}, e
		}
	} else if err != nil {
		return tls.Certificate{}, err
	}
	cert, err := tls.X509KeyPair(raw, raw)
	if err != nil {
		return cert, fmt.Errorf("identity is damaged; refusing to replace a paired identity: %w", err)
	}
	return cert, nil
}
func readConfig(dir string) (Config, error) {
	var c Config
	raw, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		name, e := randomDeviceName()
		if e != nil {
			return c, e
		}
		c = Config{Version: 1, Name: name, Shared: defaultSettings(), Preferences: Preferences{Theme: "system", StartAtLogin: true}}
		// Persist at first launch, even if no pairing or settings changes occur.
		if e = writeJSON(filepath.Join(dir, "state.json"), c); e != nil {
			return Config{}, e
		}
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("state is damaged; restore state.json from a backup: %w", err)
	}
	if c.Version != 1 || c.Peer != nil && !validID(c.Peer.ID) {
		return c, errors.New("unsupported or invalid saved state")
	}
	c.Name = cleanName(c.Name)
	c.Shared = normalizedShared(c.Shared)
	if c.Preferences.Theme == "" {
		c.Preferences = Preferences{Theme: "system", StartAtLogin: true}
	}
	return c, nil
}
func defaultSettings() Shared {
	return Shared{Side: "left", Enabled: true, AudioVolume: 100, AudioBufferMS: 80, ReturnKey: "F12"}
}
func normalizedShared(s Shared) Shared {
	if s.ReturnKey == "" {
		s.ReturnKey = "F12"
	}
	if s.AudioBufferMS == 0 {
		s.AudioBufferMS = 80
		if s.AudioTarget == "" && s.AudioVolume == 0 {
			s.AudioVolume = 100
		}
	}
	return s
}
func newer(a, b Shared) Shared {
	if a.Revision > b.Revision || a.Revision == b.Revision && a.Actor > b.Actor {
		return a
	}
	return b
}
