package utils

import (
	"os"
	"path/filepath"
	"strconv"
)

// Function to read the namespace link
func readNamespaceLink(pid string, nsType string) (string, error) {
	nsPath := filepath.Join("/proc", pid, "ns", nsType)
	link, err := os.Readlink(nsPath)
	if err != nil {
		return "", err
	}
	return link, nil
}

func ListNetNamespaces() ([]string, error) {
	procPath := "/proc"
	files, err := os.ReadDir(procPath)
	if err != nil {
		return nil, err
	}

	nsMap := make(map[string][]int)

	for _, file := range files {
		if file.IsDir() {
			elem := file.Name()
			if pid, err := strconv.Atoi(elem); err == nil {
				nsLink, err := readNamespaceLink(elem, "net")
				if err == nil {
					nsMap[nsLink] = append(nsMap[nsLink], pid)
				}
			}
		}
	}

	nsIDs := []string{}
	for nsID := range nsMap {
		nsIDs = append(nsIDs, nsID)
	}
	return nsIDs, nil
}

func ListDifferentNetNamespacePids() ([]int, error) {
	procPath := "/proc"
	files, err := os.ReadDir(procPath)
	if err != nil {
		return nil, err
	}

	nsMap := make(map[string]int)

	for _, file := range files {
		if file.IsDir() {
			elem := file.Name()
			if pid, err := strconv.Atoi(elem); err == nil {
				nsLink, err := readNamespaceLink(elem, "net")
				if err == nil {
					if pid == 1 {
						nsMap[nsLink] = pid
					} else {
						_, ok := nsMap[nsLink]
						if !ok {
							nsMap[nsLink] = pid
						}
					}
				}
			}
		}
	}

	pids := []int{}
	for _, pid := range nsMap {
		pids = append(pids, pid)
	}
	return pids, nil
}
