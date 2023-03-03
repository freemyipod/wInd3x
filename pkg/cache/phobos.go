package cache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"howett.net/plist"

	"github.com/freemyipod/wInd3x/pkg/devices"
)

type jingle struct {
	MobileDeviceSoftware map[string]mobileDeviceSoftwareVersion `plist:"MobileDeviceSoftwareVersionsByVersion"`
	IPodSoftwareVersions map[string]iPodSoftwareVersion         `plist:"iPodSoftwareVersions"`
}

type mobileDeviceSoftwareVersion struct {
	RecoverySoftware struct {
		WTF      map[string]recoverySoftware `plist:"WTF"`
		Firmware struct {
			DFU map[string]recoverySoftware `plist:"DFU"`
		} `plist:"Firmware"`
	} `plist:"RecoverySoftwareVersions"`
}

type recoverySoftware struct {
	FirmwareURL string
}

type iPodSoftwareVersion struct {
	UpdaterFamilyID int    `plist:"UpdaterFamilyID"`
	FirmwareURL     string `plist:"FirmwareURL"`
}

const jingleURL = "http://ax.phobos.apple.com.edgesuite.net/WebObjects/MZStore.woa/wa/com.apple.jingle.appserver.client.MZITunesClientCheck/version"

func getJingle() (*jingle, error) {
	fspath := pathFor(nil, PayloadKindJingleXML)
	var bytes []byte
	if _, err := os.Stat(fspath); err == nil {
		bytes, _ = os.ReadFile(fspath)
	}
	if bytes == nil {
		glog.Infof("Downloading iTunes XML...")
		resp, err := http.Get(jingleURL)
		if err != nil {
			return nil, fmt.Errorf("could not download iTunes XML: %w", err)
		}
		bytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("could not download iTunes XML: %w", err)
		}

		os.MkdirAll(filepath.Dir(fspath), 0755)
		if err := os.WriteFile(fspath, bytes, 0644); err != nil {
			glog.Errorf("Could not save iTunes XML cache: %v", err)
		}
	} else {
		glog.Infof("Using iTunes XML cache at %s", fspath)
	}

	var res jingle
	if _, err := plist.Unmarshal(bytes, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func RecoveryFirmwareDFUURL(dev devices.Kind) (string, error) {
	j, err := getJingle()
	if err != nil {
		return "", err
	}

	pidext := int(dev.Description().Kind.Description().PIDs[devices.WTF]) << 16
	k2 := fmt.Sprintf("%d", pidext)

	for _, v := range j.MobileDeviceSoftware {
		if rs, ok := v.RecoverySoftware.Firmware.DFU[k2]; ok {
			return rs.FirmwareURL, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func RecoveryWTFURL(dev devices.Kind) (string, error) {
	j, err := getJingle()
	if err != nil {
		return "", err
	}

	pidext := int(dev.Description().PIDs[devices.DFU]) << 16
	k2 := fmt.Sprintf("%d", pidext)

	for _, v := range j.MobileDeviceSoftware {
		if rs, ok := v.RecoverySoftware.WTF[k2]; ok {
			return rs.FirmwareURL, nil
		}
	}
	return "", fmt.Errorf("not found")
}

func FirmwareURL(dev devices.Kind) (string, error) {
	j, err := getJingle()
	if err != nil {
		return "", err
	}

	for _, isv := range j.IPodSoftwareVersions {
		if isv.UpdaterFamilyID != dev.Description().UpdaterFamilyID {
			continue
		}
		return isv.FirmwareURL, nil
	}
	return "", fmt.Errorf("not found")
}
