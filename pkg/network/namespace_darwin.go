//go:build darwin
// +build darwin

/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */

package network

// Function to read the namespace link
func readNamespaceLink(pid string, nsType string) (string, error) {
	// Darwin stub implementation
	// Network namespaces are not available on Darwin
	return "", nil
}

func ListNetNamespaces() ([]string, error) {
	// Darwin stub implementation
	// Network namespaces are not available on Darwin
	return []string{}, nil
}

func ListDifferentNetNamespacePids() ([]int, error) {
	// Darwin stub implementation
	// Network namespaces are not available on Darwin
	return []int{}, nil
}
