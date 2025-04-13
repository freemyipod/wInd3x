package cache

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"

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

const jingleURL = "https://itunes.apple.com/WebObjects/MZStore.woa/wa/com.apple.jingle.appserver.client.MZITunesClientCheck/version"

var (
	extraFirmwareVersions = map[devices.Kind]map[string]string{
		devices.Nano3: {
			"1.0.1": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-3878.20070914.P0omB/iPod_26.1.0.1.ipsw",
			"1.0.2": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-3930.20071005.94rVg/iPod_26.1.0.2.ipsw",
			"1.0.3": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-3941.20071115.Hngr4/iPod_26.1.0.3.ipsw",
			"1.1":   "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-4011.20080115.Gh5yt/iPod_26.1.1.ipsw",
			"1.1.2": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-4276.20080430.Gbjt5/iPod_26.1.1.2.ipsw",
			"1.1.3": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-5164.20080722.hnt3A/iPod_26.1.1.3.ipsw",
		},
		devices.Nano5: {
			"1.0.1": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-7165.20090909.AzPKm/iPod_1.0.1_34A10006.ipsw",
		},
		devices.Nano6: {
			"1.0": "http://appldnld.apple.com/iPod/SBML/osx/bundles/061-9054.20100907.VKPt5/iPod_1.0_36A00403.ipsw",
		},
	}
)

func GetFirmwareVersions(dk devices.Kind) []string {
	var res []string
	if extra, ok := extraFirmwareVersions[dk]; ok {
		for k, _ := range extra {
			res = append(res, k)
		}
	}
	sort.Strings(res)
	res = append(res, "current")
	return res
}

var FirmwareVersionOverrides map[devices.Kind]string

func getJingle() (*jingle, error) {
	fspath := pathFor(nil, PayloadKindJingleXML, "")
	var bytes []byte
	exists, err := Store.Exists(fspath)
	if err != nil {
		return nil, err
	}
	if exists {
		slog.Info("Jingle: Using cached XML....")
		bytes, _ = Store.ReadFile(fspath)
	}
	if bytes == nil {
		slog.Info("Jingle: Downloading XML...")
		resp, err := http.Get(jingleURL)
		if err != nil {
			return nil, fmt.Errorf("could not download iTunes XML: %w", err)
		}
		bytes, err = io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("could not download iTunes XML: %w", err)
		}

		if err := Store.WriteFile(fspath, bytes); err != nil {
			slog.Error("Could not save iTunes XML cache", "err", err)
		}
	}
	slog.Info("Jingle: Got XML!")

	var res jingle
	if _, err := plist.Unmarshal(bytes, &res); err != nil {
		return nil, err
	}
	slog.Info("Jingle: Unmarshaled.")
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
	if version, ok := FirmwareVersionOverrides[dev]; ok {
		if version != "current" {
			if extra, ok := extraFirmwareVersions[dev]; ok {
				if url, ok := extra[version]; ok {
					return url, nil
				}
			}
			return "", fmt.Errorf("firmware IPSW override specified, but invalid")
		}
	}

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

func urlForKind(pk PayloadKind, dk devices.Kind) (string, error) {
	switch pk {
	case PayloadKindWTFUpstream:
		return RecoveryWTFURL(dk)
	case PayloadKindRecoveryUpstream:
		return RecoveryFirmwareDFUURL(dk)
	case PayloadKindFirmwareUpstream, PayloadKindBootloaderUpstream, PayloadKindRetailOSUpstream, PayloadKindDiagsUpstream:
		return FirmwareURL(dk)
	default:
		return "", nil
	}
}
