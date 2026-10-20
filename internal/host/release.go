package host

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/erranliu/SteerRunner/internal/contracts"
)

func executableDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func selectedService(info contracts.ServiceInfo, executable string) (contracts.ServiceInfo, *contracts.Fault) {
	_, err := executableDigest(executable)
	if err != nil {
		return contracts.ServiceInfo{}, contracts.Fail(4, "service_unavailable", "Cannot verify the selected service executable.")
	}
	if info.Protocol != contracts.Protocol || info.ExecutableSHA256 == "" || info.InstanceID == "" || info.ResourceDigest == "" {
		return contracts.ServiceInfo{}, contracts.Fail(5, "directory_conflict", "The original service protocol or identity is unverified.")
	}
	return info, nil
}
