//go:build windows

package config

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32            = windows.NewLazySystemDLL("crypt32.dll")
	kernel32           = windows.NewLazySystemDLL("kernel32.dll")
	cryptProtectData   = crypt32.NewProc("CryptProtectData")
	cryptUnprotectData = crypt32.NewProc("CryptUnprotectData")
	localFree          = kernel32.NewProc("LocalFree")
)

func saveTranscriptionSecret(path, secret string) error {
	plain := []byte(secret)
	if len(plain) == 0 {
		return nil
	}
	in := dataBlob{cbData: uint32(len(plain)), pbData: &plain[0]}
	var out dataBlob
	r1, _, callErr := cryptProtectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if r1 == 0 {
		return fmt.Errorf("DPAPI 加密失败: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	protected := append([]byte(nil), unsafe.Slice(out.pbData, int(out.cbData))...)
	return os.WriteFile(path, protected, 0600)
}

func loadTranscriptionSecret(path string) (string, error) {
	protected, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(protected) == 0 {
		return "", nil
	}
	in := dataBlob{cbData: uint32(len(protected)), pbData: &protected[0]}
	var out dataBlob
	r1, _, callErr := cryptUnprotectData.Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 0, uintptr(unsafe.Pointer(&out)))
	if r1 == 0 {
		return "", fmt.Errorf("DPAPI 解密失败: %w", callErr)
	}
	defer localFree.Call(uintptr(unsafe.Pointer(out.pbData)))
	return string(unsafe.Slice(out.pbData, int(out.cbData))), nil
}
