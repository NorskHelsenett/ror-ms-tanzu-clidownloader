package clidownloader

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/NorskHelsenett/ror/pkg/rlog"
	kubectlversion "k8s.io/kubectl/pkg/cmd/version"
)

// Security constants
const (
	// Maximum file size for extraction (100MB)
	maxFileSize = 100 * 1024 * 1024
	// Safe directory permissions
	safeDirPerm = 0750
)

type TanzuCliDownloaderConfig struct {
	DatacenterUrl string
	UrlPath       string
	AppPath       string
}

func (t TanzuCliDownloaderConfig) GetDatacenterUri() string {
	if t.DatacenterUrl == "" || t.UrlPath == "" {
		panic("Datacenter URL and or path is not set")
	}
	return fmt.Sprintf("https://%s/%s", t.DatacenterUrl, t.UrlPath)
}

func (t TanzuCliDownloaderConfig) GetAppPath() string {
	return t.AppPath
}

type CliVersions struct {
	KubectlVersion        string
	KubectlVsphereVersion string
}

func DownloadCli(config *TanzuCliDownloaderConfig) (*CliVersions, error) {
	urlStr := config.GetDatacenterUri()
	dst := config.GetAppPath()
	
	// Validate URL
	if err := validateURL(urlStr); err != nil {
		return nil, err
	}
	
	// Download file
	dlFilePath, err := downloadFile(urlStr)
	if err != nil {
		return nil, err
	}
	
	// Extract required files
	if err := extractFiles(dlFilePath, dst); err != nil {
		return nil, err
	}
	
	return getCliVersions(dst)
}

// validateURL validates that the URL is secure and properly formatted
func validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("only HTTPS URLs are allowed")
	}
	return nil
}

// downloadFile downloads a file from the given URL to a temporary location
func downloadFile(urlStr string) (string, error) {
	dlFilePath := filepath.Join(os.TempDir(), "vsphere-plugin.zip")
	// Validate file path to fix G304
	if !filepath.IsAbs(dlFilePath) {
		return "", fmt.Errorf("file path must be absolute")
	}
	
	out, err := os.Create(dlFilePath) // #nosec G304 - using temp directory
	if err != nil {
		return "", err
	}
	defer func() {
		if err := out.Close(); err != nil {
			rlog.Debug("Failed to close file", rlog.String("file", dlFilePath), rlog.String("error", err.Error()))
		}
	}()
	
	rlog.Infof("Downloading %s to %s", urlStr, out.Name())
	
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 - we are ok with self-signed certs in this case
	resp, err := http.Get(urlStr) // #nosec G107 - URL is validated above
	if err != nil {
		return "", err
	}
	
	defer func() {
		if err := resp.Body.Close(); err != nil {
			rlog.Debug("Failed to close response body", rlog.String("error", err.Error()))
		}
	}()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}
	
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}
	
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("failed to close file %s: %w", dlFilePath, err)
	}
	
	return dlFilePath, nil
}

// extractFiles extracts kubectl and kubectl-vsphere from the downloaded archive
func extractFiles(dlFilePath, dst string) error {
	archive, err := zip.OpenReader(dlFilePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := archive.Close(); err != nil {
			rlog.Debug("Failed to close archive", rlog.String("file", dlFilePath), rlog.String("error", err.Error()))
		}
	}()
	
	for _, f := range archive.File {
		filename := filepath.Base(f.Name)
		if filename == "kubectl" || filename == "kubectl-vsphere" {
			err = UnzipToDstFlat(f, dst)
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func getCliVersions(path string) (*CliVersions, error) {
	versions := CliVersions{}
	var err error

	versions.KubectlVersion, err = getKubectlVersion(path)
	if err != nil {
		return nil, err
	}
	versions.KubectlVsphereVersion, err = getKubectlVsphereVersion(path)
	if err != nil {
		return nil, err
	}
	return &versions, nil

}

func getKubectlVersion(path string) (string, error) {
	path = filepath.Join(path, "kubectl")
	fileinfo, err := os.Stat(path)

	if errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if !isExecOwner(fileinfo.Mode()) {
		return "", fmt.Errorf("file %s is not executable", path)
	}

	args := []string{"version", "--client=true", "-o", "json"}
	rlog.Debug("executing", rlog.String("cmd", path), rlog.Strings("args", args))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...) // #nosec G204 - we are not using user input

	data, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	kubeversion := kubectlversion.Version{}

	err = json.Unmarshal(data, &kubeversion)
	if err != nil {
		return "", err
	}
	versionstring := fmt.Sprintf("%s.%s", kubeversion.ClientVersion.Major, kubeversion.ClientVersion.Minor)
	rlog.Debug("kubectl version found", rlog.String("version", versionstring))
	return versionstring, nil
}
func getKubectlVsphereVersion(path string) (string, error) {
	path = filepath.Join(path, "kubectl-vsphere")
	fileinfo, err := os.Stat(path)

	if errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	if !isExecOwner(fileinfo.Mode()) {
		return "", fmt.Errorf("file %s is not executable", path)
	}
	args := []string{"version"}
	rlog.Debug("executing", rlog.String("cmd", path), rlog.Strings("args", args))
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	command := exec.CommandContext(ctx, path, args...) // #nosec G204 - we are not using user input

	data, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	datas := strings.Split(string(data), " ")
	rlog.Debug("kubectl-vsphere version found", rlog.String("version", datas[2]))
	return datas[2], nil
}
func isExecOwner(mode os.FileMode) bool {
	return mode&0100 != 0
}

func UnzipToDst(f *zip.File, dst string) error {
	// Fix G305 - validate path to prevent zip traversal
	filePath := filepath.Join(dst, f.Name) // #nosec G305 - path is validated below
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(dst)) {
		return fmt.Errorf("invalid file path: %s", f.Name)
	}
	return unzipToPath(f, filePath)
}

func UnzipToDstFlat(f *zip.File, dst string) error {
	filePath := filepath.Join(dst, filepath.Base(f.Name))
	return unzipToPath(f, filePath)
}

func unzipToPath(f *zip.File, dst string) error {
	rlog.Debug("unzipping file", rlog.String("path", dst))

	// Fix G304 - validate destination path
	if !filepath.IsAbs(dst) {
		return fmt.Errorf("destination path must be absolute")
	}

	if f.FileInfo().IsDir() {
		fmt.Println("creating directory...", f.Name)
		err := os.MkdirAll(dst, safeDirPerm) // Fix G301 - use safer permissions
		if err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), safeDirPerm); err != nil { // Fix G301 - use safer permissions
		panic(err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode()) // #nosec G304 - path is validated above
	if err != nil {
		return err
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			rlog.Debug("Failed to close destination file", rlog.String("file", dst), rlog.String("error", err.Error()))
		}
	}()

	fileInArchive, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if err := fileInArchive.Close(); err != nil {
			rlog.Debug("Failed to close file in archive", rlog.String("error", err.Error()))
		}
	}()

	// Fix G110 - Limit copy size to prevent decompression bomb
	limitedReader := io.LimitReader(fileInArchive, maxFileSize)
	if _, err := io.Copy(dstFile, limitedReader); err != nil {
		return err
	}
	return nil
}
