package update

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

const baseURL = "https://github.com/gwillem/whip/releases/latest/download/whip"

func getUnameOsArch() (string, string) {
	var unameS, unameM string

	unameS = runtime.GOOS
	unameS = string(unameS[0]-32) + unameS[1:]

	switch runtime.GOARCH {
	case "amd64":
		unameM = "x86_64"
	case "386":
		unameM = "i386"
	case "arm64":
		if runtime.GOOS == "darwin" {
			unameM = "arm64"
		} else {
			unameM = "aarch64"
		}
	default:
		unameM = runtime.GOARCH
	}

	return unameS, unameM
}

func Run(oldver string) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Resolve symlinks in currentExe
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("failed to resolve symlinks for current executable: %w", err)
	}

	unameS, unameM := getUnameOsArch()
	downloadURL := fmt.Sprintf("%s-%s-%s.gz", baseURL, unameS, unameM)

	oldETag, err := fetchETag()
	if err != nil {
		return fmt.Errorf("failed to fetch cached ETag: %w", err)
	}

	// Conditionally download the new version based on the ETag
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	if oldETag != "" {
		req.Header.Set("If-None-Match", oldETag)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download new version: %w", err)
	}
	defer resp.Body.Close()

	if err := storeETag(resp.Header.Get("ETag")); err != nil {
		return fmt.Errorf("failed to store ETag: %w", err)
	}

	if resp.StatusCode == http.StatusNotModified {
		fmt.Println("Already up to date")
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download new version: HTTP status %d", resp.StatusCode)
	}

	// Create a temporary file to store the download in the same directory as the executable
	tempFile, err := os.CreateTemp(filepath.Dir(currentExe), "whip-update")
	if err != nil {
		return fmt.Errorf("failed to create temporary file in executable directory: %w", err)
	}
	defer os.Remove(tempFile.Name())

	// Decompress and write the downloaded content to the temporary file
	gzipReader, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	_, err = io.Copy(tempFile, gzipReader)
	if err != nil {
		return fmt.Errorf("failed to write decompressed content: %w", err)
	}
	tempFile.Close()

	// Get the permissions of the current executable
	currentInfo, err := os.Stat(currentExe)
	if err != nil {
		return fmt.Errorf("failed to get current executable info: %w", err)
	}

	// Copy the permissions to the temporary file
	err = os.Chmod(tempFile.Name(), currentInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}

	// Compare the files
	if filesAreIdentical(currentExe, tempFile.Name()) {
		fmt.Println("Already up to date.")
		return nil
	}

	// test new executable
	if err := exec.Command(tempFile.Name(), "version").Run(); err != nil {
		return fmt.Errorf("new version failed to run: %w", err)
	}

	// Replace the current executable with the new one
	err = os.Rename(tempFile.Name(), currentExe)
	if err != nil {
		return fmt.Errorf("failed to replace current executable: %w", err)
	}

	// Replace the current process with the new version
	fmt.Printf("whip %s ==> ", oldver)
	err = syscall.Exec(currentExe, []string{currentExe, "version"}, os.Environ())
	if err != nil {
		return fmt.Errorf("failed to execute new version: %w", err)
	}
	return nil // never reached
}

func filesAreIdentical(file1, file2 string) bool {
	// Open both files
	f1, err := os.Open(file1)
	if err != nil {
		return false
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		return false
	}
	defer f2.Close()

	// Create hash objects
	h1 := sha256.New()
	h2 := sha256.New()

	// Copy file contents to hash objects
	if _, err := io.Copy(h1, f1); err != nil {
		return false
	}
	if _, err := io.Copy(h2, f2); err != nil {
		return false
	}

	// Compare the hashes
	return bytes.Equal(h1.Sum(nil), h2.Sum(nil))
}

func storeETag(etag string) error {
	if etag == "" {
		return nil
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get user cache directory: %w", err)
	}
	etagFile := filepath.Join(cacheDir, "whip", "update-etag")
	if err := os.MkdirAll(filepath.Dir(etagFile), 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := os.WriteFile(etagFile, []byte(etag), 0o644); err != nil {
		return fmt.Errorf("failed to write ETag to cache: %w", err)
	}
	return nil
}

func fetchETag() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user cache directory: %w", err)
	}
	etagFile := filepath.Join(cacheDir, "whip", "update-etag")
	etag, err := os.ReadFile(etagFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read ETag from cache: %w", err)
	}
	return string(etag), nil
}
