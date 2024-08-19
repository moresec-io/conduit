/*
 * Apache License 2.0
 *
 * Copyright (c) 2022, Moresec Inc.
 * All rights reserved.
 */
package client

func (client *Client) setIPSet() error {
	err := client.repo.InitIPSet()
	if err != nil {
		return err
	}
	return nil
}
