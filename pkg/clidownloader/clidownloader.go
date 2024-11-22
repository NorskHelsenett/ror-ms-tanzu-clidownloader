package clidownloader

import (
	"archive/zip"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/NorskHelsenett/ror/pkg/rlog"
)

func DownloadCli(datacenterUrl string) error {
	url := fmt.Sprintf("https://%s/%s", datacenterUrl, "wcp/plugin/linux-amd64/vsphere-plugin.zip")
	dlFilePath := filepath.Join(os.TempDir(), "vsphere-plugin.zip")
	dst := "app"

	out, err := os.Create(dlFilePath)
	if err != nil {
		return err
	}
	defer out.Close()

	rlog.Infof("Downloading %s to %s", url, out.Name())

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	out.Close()

	archive, err := zip.OpenReader(dlFilePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	for _, f := range archive.File {
		filename := filepath.Base(f.Name)
		if filename == "kubectl" || filename == "kubectl-vsphere" {
			err = unzipToDstFlat(f, dst)
			if err != nil {
				return err
			}
		}
	}
	return nil

	// Download the tanzu cli
}
func unzipToDst(f *zip.File, dst string) error {
	filePath := filepath.Join(dst, f.Name)
	return unzipToPath(f, filePath)
}

func unzipToDstFlat(f *zip.File, dst string) error {
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
