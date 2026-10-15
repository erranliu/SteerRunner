package host

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/erranliu/SteerRunner/internal/contracts"
	"github.com/erranliu/SteerRunner/internal/resources"
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
	digest, err := executableDigest(executable)
	if err != nil {
		return contracts.ServiceInfo{}, contracts.Fail(4, "service_unavailable", "Cannot verify the selected service executable.")
	}
	if info.ExecutableSHA256 != digest || info.Version != resources.Version || info.Protocol != contracts.Protocol || info.ResourceDigest != resources.Digest() {
		return contracts.ServiceInfo{}, contracts.Fail(5, "directory_conflict", "The original service uses a different executable release; stop it with its original CLI before selecting another release.")
	}
	return info, nil
}
