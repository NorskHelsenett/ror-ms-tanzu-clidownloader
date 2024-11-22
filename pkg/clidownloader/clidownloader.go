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

type CliVersions struct {
	KubectlVersion        string
	KubectlVsphereVersion string
}

func DownloadCli(datacenterUrl string) (*CliVersions, error) {
	url := fmt.Sprintf("https://%s/%s", datacenterUrl, "wcp/plugin/linux-amd64/vsphere-plugin.zip")
	dlFilePath := filepath.Join(os.TempDir(), "vsphere-plugin.zip")
	dst := "app"

	out, err := os.Create(dlFilePath)
	if err != nil {
		return nil, err
	}
	defer out.Close()

	rlog.Infof("Downloading %s to %s", url, out.Name())

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return nil, err
	}

	out.Close()

	archive, err := zip.OpenReader(dlFilePath)
	if err != nil {
		return nil, err
	}
	defer archive.Close()

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...) // #nosec G204 - we are not using user input

	data, err := command.CombinedOutput()
	if err != nil {
		return "", err
	}
	datas := strings.Split(string(data), " ")

	return datas[2], nil
}
func isExecOwner(mode os.FileMode) bool {
	return mode&0100 != 0
}

func UnzipToDst(f *zip.File, dst string) error {
	filePath := filepath.Join(dst, f.Name)
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
		err := os.MkdirAll(dst, os.ModePerm)
		if err != nil {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), os.ModePerm); err != nil {
		panic(err)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	fileInArchive, err := f.Open()
	if err != nil {
		return err
	}
	defer fileInArchive.Close()

	if _, err := io.Copy(dstFile, fileInArchive); err != nil {
		return err
	}
	return nil
}
