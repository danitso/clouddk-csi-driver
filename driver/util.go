/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/. */

package driver

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
)

const (
	rtNetworkStorage = "NS"
	rtVolumes        = "VOLUMES"
)

var (
	serverPackageIDs = []string{
		"ac949a1cb4731d",
		"89833c1dfa7010",
		"0469d586374e76",
		"e991abd8ef15c7",
		"489b7df86d4b76",
		"9559dbb4b71c45",
		"ebf313a9994c1e",
		"86fa7f6209ba2a",
		"25848db6009838",
		"115f1d99e8e9e4",
	}
)

// debugCloudAction writes a debug message to the log.
func debugCloudAction(resourceType string, format string, v ...interface{}) {
	log.Printf(fmt.Sprintf("[%s] ", resourceType)+format, v...)
}

// getPackageID returns a server package id based on hardware requirements.
func getPackageID(memory, processors int) (id *string, err error) {
	memoryPackageIndex := -1

	if memory <= 512 {
		memoryPackageIndex = 0
	} else if memory <= 1024 {
		memoryPackageIndex = 1
	} else if memory <= 2048 {
		memoryPackageIndex = 2
	} else if memory <= 4096 {
		memoryPackageIndex = 3
	} else if memory <= 6144 {
		memoryPackageIndex = 4
	} else if memory <= 8192 {
		memoryPackageIndex = 5
	} else if memory <= 16384 {
		memoryPackageIndex = 6
	} else if memory <= 32768 {
		memoryPackageIndex = 7
	} else if memory <= 65536 {
		memoryPackageIndex = 8
	} else if memory <= 98304 {
		memoryPackageIndex = 9
	} else {
		return nil, fmt.Errorf("No supported packages provide %d MB of memory", memory)
	}

	processorsPackageIndex := -1

	if processors <= 1 {
		processorsPackageIndex = 0
	} else if processors <= 2 {
		processorsPackageIndex = 3
	} else if processors <= 3 {
		processorsPackageIndex = 4
	} else if processors <= 4 {
		processorsPackageIndex = 5
	} else if processors <= 6 {
		processorsPackageIndex = 6
	} else if processors <= 8 {
		processorsPackageIndex = 7
	} else if processors <= 10 {
		processorsPackageIndex = 8
	} else if processors <= 12 {
		processorsPackageIndex = 9
	} else {
		return nil, fmt.Errorf("No supported packages provide %d processors", processors)
	}

	packageIndex := int(math.Max(float64(memoryPackageIndex), float64(processorsPackageIndex)))

	if packageIndex < 0 || packageIndex >= len(serverPackageIDs) {
		return nil, fmt.Errorf("Invalid package index %d", packageIndex)
	}

	return &serverPackageIDs[packageIndex], nil
}

// getRandomPassword generates a random password of a fixed length.
func getRandomPassword(length int) string {
	var b strings.Builder

	chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")

	for i := 0; i < length; i++ {
		b.WriteRune(chars[rand.Intn(len(chars))])
	}

	return b.String()
}

// trimProviderID removes the provider name from the id.
func trimProviderID(id string) string {
	return strings.TrimPrefix(id, "clouddk://")
}
