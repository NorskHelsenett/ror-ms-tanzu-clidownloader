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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/NorskHelsenett/ror/pkg/rlog"
	kubectlversion "k8s.io/kubectl/pkg/cmd/version"
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
	url := config.GetDatacenterUri()
	dlFilePath := filepath.Join(os.TempDir(), "vsphere-plugin.zip")
	dst := config.GetAppPath()

	// #nosec G304 - dlFilePath is constructed from os.TempDir() and a fixed filename, not user input
	out, err := os.Create(dlFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := out.Close(); closeErr != nil {
			rlog.Error("failed to close file", closeErr)
		}
	}()

	rlog.Infof("Downloading %s to %s", url, out.Name())

	// #nosec G402 - InsecureSkipVerify is required for internal datacenter with self-signed certificates
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	// #nosec G107 - URL is constructed from configuration, not user input
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			rlog.Error("failed to close response body", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return nil, err
	}

	if err = out.Close(); err != nil {
		return nil, err
	}

	archive, err := zip.OpenReader(dlFilePath)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			rlog.Error("failed to close archive", closeErr)
		}
	}()

	for _, f := range archive.File {
		filename := filepath.Base(f.Name)
		if filename == "kubectl" || filename == "kubectl-vsphere" {
			err = UnzipToDstFlat(f, dst)
			if err != nil {
				return nil, err
			}
		}
	}

	return getCliVersions(dst)
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
	// #nosec G305 - Path traversal is validated in the next lines
	filePath := filepath.Join(dst, f.Name)
	// Validate that the file path doesn't escape the destination directory
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(dst)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", filePath)
	}
	return unzipToPath(f, filePath)
}

func UnzipToDstFlat(f *zip.File, dst string) error {
	filePath := filepath.Join(dst, filepath.Base(f.Name))
	return unzipToPath(f, filePath)
}

func unzipToPath(f *zip.File, dst string) error {
	rlog.Debug("unzipping file", rlog.String("path", dst))

	if f.FileInfo().IsDir() {
		fmt.Println("creating directory...", f.Name)
		err := os.MkdirAll(dst, 0750)
		if err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		panic(err)
	}

	// #nosec G304 - dst is validated by caller to prevent path traversal
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil {
			rlog.Error("failed to close destination file", closeErr)
		}
	}()

	fileInArchive, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := fileInArchive.Close(); closeErr != nil {
			rlog.Error("failed to close file in archive", closeErr)
		}
	}()

	// Limit extraction to 100MB to prevent decompression bombs
	const maxSize = 100 * 1024 * 1024 // 100MB
	// #nosec G110 - Limited to maxSize to prevent decompression bomb attacks
	if _, err := io.Copy(dstFile, io.LimitReader(fileInArchive, maxSize)); err != nil {
		return err
	}
	return nil
}
